package kafka

// Topic names — single source of truth.
// Partitioned by symbol key so all orders for BTC-USD
// always land on the same partition → same engine goroutine.
const (
	TopicOrders      = "orders"
	TopicTrades      = "trades"
	TopicOrderEvents = "order-events" // status updates: filled, cancelled, partial
	TopicMarketData  = "market-data"  // OHLCV updates for the market data service
)

// ConsumerGroups — each service has its own group ID so they
// receive independent copies of every message.
const (
	GroupEngine     = "ome-engine"     // matching engine consumes orders
	GroupMarketData = "ome-marketdata" // market data service consumes trades
	GroupWebSocket  = "ome-websocket"  // WebSocket hub consumes trades + order-events
)
