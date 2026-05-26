package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Im-Manav/ome/internal/ports"
	apperrors "github.com/Im-Manav/ome/pkg/errors"
	"github.com/Im-Manav/ome/pkg/logger"
	"github.com/Im-Manav/ome/pkg/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type OrderService struct {
	orderRepo ports.OrderRepository
	tradeRepo ports.TradeRepository
	publisher ports.EventPublisher
	cache     ports.Cache
	broadcast ports.Broadcaster
}

func NewOrderService(
	orderRepo ports.OrderRepository,
	tradeRepo ports.TradeRepository,
	publisher ports.EventPublisher,
	cache ports.Cache,
	broadcast ports.Broadcaster,
) *OrderService {
	return &OrderService{
		orderRepo: orderRepo,
		tradeRepo: tradeRepo,
		publisher: publisher,
		cache:     cache,
		broadcast: broadcast,
	}
}

func (s *OrderService) PlaceOrder(
	ctx context.Context,
	req models.PlaceOrderRequest,
	userID uuid.UUID,
) (*models.PlaceOrderResponse, error) {
	if err := validateOrderRequest(req); err != nil {
		return nil, err
	}

	order := models.Order{
		ID:           uuid.New(),
		UserID:       userID,
		Symbol:       req.Symbol,
		Side:         req.Side,
		Type:         req.Type,
		Price:        req.Price,
		Quantity:     req.Quantity,
		FilledQty:    0,
		RemainingQty: req.Quantity,
		Status:       models.StatusOpen,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.orderRepo.SaveOrder(&order); err != nil {
		return nil, fmt.Errorf("persist order: %w", err)
	}

	logger.Info("order saved",
		zap.String("order_id", order.ID.String()),
		zap.String("symbol", order.Symbol),
		zap.String("side", order.Side.String()),
	)

	if err := s.publisher.PublishOrder(ctx, order); err != nil {
		// Kafka publish failed — mark the order as rejected in DB
		// so the user isn't left with a phantom open order
		order.Status = models.StatusRejected
		_ = s.orderRepo.UpdateOrder(&order)
		return nil, fmt.Errorf("publish order: %w", err)
	}

	return &models.PlaceOrderResponse{
		Order:  order,
		Trades: nil, // trades arrive async via WebSocket
	}, nil
}

func (s *OrderService) CancelOrder(
	ctx context.Context,
	orderID uuid.UUID,
	userID uuid.UUID,
) error {
	order, err := s.orderRepo.GetOrderByID(orderID)
	if err != nil {
		return apperrors.ErrOrderNotFound
	}
	if order.UserID != userID {
		return apperrors.ErrUnauthorized
	}
	if order.Status == models.StatusFilled {
		return apperrors.ErrOrderAlreadyFilled
	}
	if order.Status == models.StatusCancelled {
		return apperrors.ErrOrderCancelled
	}

	// Update DB first
	if err := s.orderRepo.CancelOrder(orderID); err != nil {
		return fmt.Errorf("cancel order in db: %w", err)
	}

	// Publish cancellation — the Kafka consumer will call
	// book.Cancel(orderID) to remove it from the heap
	order.Status = models.StatusCancelled
	if err := s.publisher.PublishOrderEvent(ctx, *order); err != nil {
		// Non-fatal: DB is source of truth, engine will skip
		// the cancelled order when it surfaces at the top of the heap
		logger.Error("failed to publish cancel event", logger.Err(err))
	}

	return nil
}

func (s *OrderService) GetOrderBook(
	ctx context.Context,
	symbol string,
) (*models.OrderBookSnapshot, error) {
	// Cache hit — return immediately
	snap, err := s.cache.GetOrderBookSnapshot(ctx, symbol)
	if err != nil {
		logger.Error("order book cache read failed", logger.Err(err))
	}
	if snap != nil {
		return snap, nil
	}

	// Cache miss — return empty snapshot with timestamp
	// The engine will populate the cache after the next match
	empty := &models.OrderBookSnapshot{
		Symbol:    symbol,
		Bids:      []models.OrderBookLevel{},
		Asks:      []models.OrderBookLevel{},
		Timestamp: time.Now().UTC(),
	}
	return empty, nil
}

func (s *OrderService) GetUserOrders(
	ctx context.Context,
	userID uuid.UUID,
) ([]*models.Order, error) {
	orders, err := s.orderRepo.GetOrdersByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("get user orders: %w", err)
	}
	return orders, nil
}

func (s *OrderService) GetRecentTrades(
	ctx context.Context,
	symbol string,
	limit int,
) ([]models.Trade, error) {
	if limit <= 100 || limit > 100 {
		limit = 50
	}
	trades, err := s.tradeRepo.GetTradesBySymbol(symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent trades: %w", err)
	}
	return trades, nil
}

func (s *OrderService) PostMatchHandler(
	ctx context.Context,
	order models.Order,
	trades []models.Trade,
) error {
	if len(trades) > 0 {
		if err := s.tradeRepo.SaveTrades(trades); err != nil {
			logger.Error("failed to save trades", logger.Err(err))
		}
	}

	if err := s.orderRepo.UpdateOrder(&order); err != nil {
		logger.Error("failed to update order status", logger.Err(err))
	}

	for _, trade := range trades {
		event := models.TradeEvent{
			Trade:        trade,
			BuyerFilled:  order.Side == models.Buy && order.Status == models.StatusFilled,
			SellerFilled: order.Side == models.Sell && order.Status == models.StatusFilled,
		}
		s.broadcast.BroadcastTrade(event)
	}
	return nil
}

func validateOrderRequest(req models.PlaceOrderRequest) error {
	if req.Symbol == "" {
		return apperrors.ErrSymbolRequired
	}
	if req.Quantity <= 0 {
		return apperrors.ErrInvalidQuantity
	}
	if req.Type == models.Limit && req.Price <= 0 {
		return apperrors.ErrInvalidPrice
	}
	if req.Side != models.Buy && req.Side != models.Sell {
		return apperrors.ErrInvalidSide
	}
	return nil
}
