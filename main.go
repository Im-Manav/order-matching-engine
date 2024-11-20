package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"order-matching-engine/engine"

	"github.com/IBM/sarama"
)

// ConsumerGroupHandler is the custom implementation for handling Kafka messages.
type ConsumerGroupHandler struct {
	book     *engine.OrderBook
	producer sarama.AsyncProducer
}

func (h *ConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error {
	// Setup logic if needed
	return nil
}

func (h *ConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	// Cleanup logic if needed
	return nil
}

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("Message received: topic=%s, partition=%d, offset=%d\n", msg.Topic, msg.Partition, msg.Offset)

		// Deserialize the order
		var order engine.Order
		order.FromJSON(msg.Value)

		// Process the order
		trades := h.book.Process(order)
		log.Printf("Trades generated: %d\n", len(trades))

		// Send trades to Kafka topic
		for _, trade := range trades {
			rawTrade := trade.ToJSON()
			h.producer.Input() <- &sarama.ProducerMessage{
				Topic: "trades",
				Value: sarama.ByteEncoder(rawTrade),
			}
		}

		// Mark the message as processed
		session.MarkMessage(msg, "")
	}
	return nil
}

func main() {
	fmt.Println("-----------------------------------------")
	fmt.Println("        SLIM-ORDERBOOK 1.0               ")
	fmt.Println("-----------------------------------------")

	// Initialize order book
	book := &engine.OrderBook{
		Bids: make([]engine.Order, 0, 10000),
		Asks: make([]engine.Order, 0, 10000),
	}

	// Create Kafka producer
	producer := createProducer()
	defer producer.Close()

	// Create Kafka consumer group
	consumerGroup, err := sarama.NewConsumerGroup([]string{"127.0.0.1:9092"}, "orderbook-cg", createConsumerConfig())
	if err != nil {
		log.Fatalf("Error creating consumer group: %s", err)
	}
	defer consumerGroup.Close()

	// Handle system signals for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)
		<-signals
		cancel()
	}()

	// Create a consumer group handler
	handler := &ConsumerGroupHandler{book: book, producer: producer}

	// Consume messages
	for {
		err := consumerGroup.Consume(ctx, []string{"orders"}, handler)
		if err != nil {
			log.Printf("Error in consumer: %s\n", err)
		}
		if ctx.Err() != nil {
			break
		}
	}

	log.Println("Consumer group closed")
}

func createConsumerConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0 // Adjust Kafka version as needed
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Offsets.CommitInterval = time.Second
	config.Consumer.Return.Errors = true
	return config
}

func createProducer() sarama.AsyncProducer {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = false
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	producer, err := sarama.NewAsyncProducer([]string{"127.0.0.1:9092"}, config)
	if err != nil {
		log.Fatalf("Unable to connect producer to Kafka: %s", err)
	}

	// Start a goroutine to log producer errors
	go func() {
		for err := range producer.Errors() {
			log.Printf("Producer error: %s\n", err.Error())
		}
	}()
	return producer
}
