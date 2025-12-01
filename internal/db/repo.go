package db

import (
	"github.com/Im-Manav/order-matching-engine/pkg/models"
	"gorm.io/gorm"
)

// This is a repository wrapper(Helps keep the engine decoupled)

type Repo struct {
	DB *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{
		DB: db,
	}
}

func (r *Repo) SaveOrder(o *models.Order) error {
	return r.DB.Create(o).Error
}

func (r *Repo) UpdateOrder(o *models.Order) error {
	return r.DB.Save(o).Error
}

func (r *Repo) GetOrderByID(id string) (*models.Order, error) {
	var o models.Order
	if err := r.DB.First(&o, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *Repo) SaveTrade(t *models.Trade) error {
	return r.DB.Create(t).Error
}

func (r *Repo) SaveTrades(trades []*models.Trade) error {
	return r.DB.Create(&trades).Error
}

func (r *Repo) DeleteOrder(id string) error {
	return r.DB.Delete(&models.Order{}, "id = ?", id).Error
}

func (r *Repo) GetBestBid(symbol string) (*models.Order, error) {
	var order models.Order
	if err := r.DB.Where("symbol = ? AND side = ? AND quantity > 0", symbol, "BUY").Order("price DESC").First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *Repo) GetBestAsk(symbol string) (*models.Order, error) {
	var order models.Order
	if err := r.DB.Where("symbol = ? AND side =? AND quantity > 0", symbol, "SELL").Order("price ASC").First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *Repo) GetOpenOrders(symbol string, side string) ([]*models.Order, error) {
	var orders []*models.Order
	query := r.DB.Where("symbol = ? AND side = ? AND status IN ?", symbol, side, []string{"OPEN", "PARTIAL"}).Order(
		"created_at ASC",
	)
	switch side {
	case "BUY":
		query = query.Order("price DESC")
	case "SELL":
		query = query.Order("price ASC")
	}

	err := query.Find(&orders).Error
	if err != nil {
		return nil, err
	}

	return orders, nil
}
