package ports

import "github.com/Im-Manav/ome/pkg/models"

// Broadcaster pushes trade events to all connected WebSocket clients.
// Implemented by the WebSocket hub.
type Broadcaster interface {
	BroadcastTrade(event models.TradeEvent)
	BroadcastOrderBookUpdate(snapshot models.OrderBookSnapshot)
}
