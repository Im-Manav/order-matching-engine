package db

import (
	"fmt"
	"time"

	"github.com/Im-Manav/ome/pkg/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository implements all four repository ports:
//   - ports.OrderRepository
//   - ports.TradeRepository
//   - ports.OHLCVRepository
//   - ports.UserRepository
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ─── Order Repository ─────────────────────────────────────────────────────────
func (r *Repository) SaveOrder(order *models.Order) error {
	if err := r.db.Create(order).Error; err != nil {
		return fmt.Errorf("SaveOrder: %w", err)
	}
	return nil
}

func (r *Repository) UpdateOrder(order *models.Order) error {
	if err := r.db.Save(order).Error; err != nil {
		return fmt.Errorf("UpdateOrder: %w", err)
	}
	return nil
}

func (r *Repository) GetOrderByID(id uuid.UUID) (*models.Order, error) {
	var order models.Order
	if err := r.db.First(&order, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("GetOrderByID: %w", err)
	}
	return &order, nil
}

func (r *Repository) GetOpenOrdersBySymbol(symbol string) ([]*models.Order, error) {
	var orders []*models.Order
	err := r.db.Where("symbol = ? AND status IN ?", symbol, []models.OrderStatus{
		models.StatusOpen,
		models.StatusPartial,
	}).Order("created_at ASC").Find(&orders).Error
	if err != nil {
		return nil, fmt.Errorf("GetOpenOrdersBySymbol: %w", err)
	}
	return orders, nil
}

func (r *Repository) GetOrdersByUserID(userID uuid.UUID) ([]*models.Order, error) {
	var orders []*models.Order
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(100).
		Find(&orders).Error
	if err != nil {
		return nil, fmt.Errorf("GetOrdersByUserID: %w", err)
	}
	return orders, nil
}

func (r *Repository) CancelOrder(id uuid.UUID) error {
	result := r.db.Model(&models.Order{}).
		Where("id = ? AND status IN ?", id, []models.OrderStatus{
			models.StatusOpen,
			models.StatusPartial,
		}).
		Updates(map[string]any{
			"status":     models.StatusCancelled,
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("CancelOrder: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("CancelOrder: order not found or already terminal")
	}
	return nil
}

// ─── Trade Repository ─────────────────────────────────────────────────────────
func (r *Repository) SaveTrade(trade *models.Trade) error {
	if err := r.db.Create(trade).Error; err != nil {
		return fmt.Errorf("SaveTrade: %w", err)
	}
	return nil
}

func (r *Repository) SaveTrades(trades []models.Trade) error {
	if len(trades) == 0 {
		return nil
	}
	// Batch insert — one round trip regardless of how many trades
	if err := r.db.Create(&trades).Error; err != nil {
		return fmt.Errorf("SaveTrades: %w", err)
	}
	return nil
}

func (r *Repository) GetTradesBySymbol(symbol string, limit int) ([]models.Trade, error) {
	var trades []models.Trade
	err := r.db.Where("symbol = ?", symbol).
		Order("executed_at DESC").
		Limit(limit).
		Find(&trades).Error
	if err != nil {
		return nil, fmt.Errorf("GetTradesBySymbol: %w", err)
	}
	return trades, nil
}

func (r *Repository) GetTradesByUserID(userID uuid.UUID, limit int) ([]models.Trade, error) {
	var trades []models.Trade
	err := r.db.Where("buy_user_id = ? OR sell_user_id = ?", userID, userID).
		Order("executed_at DESC").
		Limit(limit).
		Find(&trades).Error
	if err != nil {
		return nil, fmt.Errorf("GetTradesByUserID: %w", err)
	}
	return trades, nil
}

// ─── OHLCV Repository ────────────────────────────────────────────────────────

func (r *Repository) UpsertOHLCV(bar *models.OHLCV) error {
	// ON CONFLICT: if a bar for this (time, symbol) already exists,
	// update high/low/close/volume — this handles out-of-order trade events
	err := r.db.Exec(`
		INSERT INTO ohlcvs (time, symbol, open, high, low, close, volume)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (time, symbol) DO UPDATE SET
			high = GREATEST(ohlcvs.high, EXCLUDED.high),
			low    = LEAST(ohlcvs.low,      EXCLUDED.low),
			close  = EXCLUDED.close,
			volume = ohlcvs.volume + EXCLUDED.volume
	`,
		bar.Time, bar.Symbol,
		bar.Open, bar.High, bar.Low, bar.Close, bar.Volume,
	).Error
	if err != nil {
		return fmt.Errorf("UpsertOHLCV: %w", err)
	}
	return nil
}

func (r *Repository) GetOHLCV(symbol, interval string, limit int) ([]models.OHLCV, error) {
	// TimescaleDB time_bucket groups trades into candles of any interval
	// e.g. interval = '1 minute', '5 minutes', '1 hour'
	var bars []models.OHLCV
	err := r.db.Raw(`
		SELECT
			time_bucket(?, time) AS time,
			symbol,
			first(open, time) AS open,
			max(high) AS high,
			min(low) AS low,
			last(close, time) AS close,
			sum(volume) AS volume
		FROM ohlcvs
		WHERE symbol = ?
		GROUP BY time_bucket(?, time), symbol
		ORDER BY time DESC
		LIMIT ?
	`, interval, symbol, interval, limit).Scan(&bars).Error
	if err != nil {
		return nil, fmt.Errorf("GetOHLCV:%w", err)
	}
	return bars, nil
}

// ─── User Repository ──────────────────────────────────────────────────────────

func (r *Repository) CreateUser(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("CreateUser: %w", err)
	}
	return nil
}

func (r *Repository) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, "email = ?", email).Error; err != nil {
		return nil, fmt.Errorf("GetUserByEmail: %w", err)
	}
	return &user, nil
}

func (r *Repository) GetUserByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("GetUserByID: %w", err)
	}
	return &user, nil
}
