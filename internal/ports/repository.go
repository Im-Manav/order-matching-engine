package ports

import (
	"github.com/Im-Manav/ome/pkg/models"
	"github.com/google/uuid"
)

// OrderRepository — all DB operations for orders
// The matching engine never calls this directly
type OrderRepository interface {
	SaveOrder(order *models.Order) error
	UpdateOrder(order models.Order) error
	GetOrderByID(id uuid.UUID) (*models.Order, error)
	GetOpenOrdersBySymbol(symbol string) ([]*models.Order, error)
	GetOrdersByUserID(userID uuid.UUID) ([]*models.Order, error)
	CancelOrder(id uuid.UUID) error
}

// TradeRepository — all DB operations for trades
type TradeRepository interface {
	SaveTrade(trade *models.Trade) error
	SaveTrades(trades []models.Trade) error
	GetTradesBySymbol(symbol string, limit int) ([]models.Trade, error)
	GetTradesByUserID(userID uuid.UUID, limit int) ([]models.Trade, error)
}

// OHLCVRepository — candlestick data (TimescaleDB)
type OHLCVRepository interface {
	UpsertOHLCV(bar *models.OHLCV) error
	GetOHLCV(symbol, interval string, limit int) ([]models.OHLCV, error)
}

// UserRepository — auth
type UserRepository interface {
	CreateUser(user *models.User) error
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id uuid.UUID) (*models.User, error)
}
