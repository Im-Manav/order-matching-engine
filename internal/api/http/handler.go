package http

import (
	"net/http"

	"github.com/Im-Manav/order-matching-engine/internal/db"
	"github.com/Im-Manav/order-matching-engine/internal/engine"
	"github.com/Im-Manav/order-matching-engine/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type HTTPHandler struct {
	orderBook *engine.OrderBook
	repo      *db.Repo
}

func NewHTTPHandler(repo *db.Repo) *HTTPHandler {
	h := &HTTPHandler{
		orderBook: engine.NewOrderBook(),
		repo:      repo,
	}
	h.RestoreOrderBook("AAPL")
	return h
}

type OrderRequest struct {
	Symbol   string  `json:"symbol" binding:"required"`
	Side     string  `json:"side" binding:"required,oneof=BUY SELL"`
	Price    float64 `json:"price" binding:"required,gt=0"`
	Quantity int64   `json:"quantity" binding:"required,gt=0"`
}

type OrderResponse struct {
	Trades  []*models.Trade `json:"trades"`
	OrderID string          `json:"order_id"`
	Message string          `json:"message"`
}

// PlaceOrder godoc
// @Summary Place a new order
// @Description Creates a buy or sell order in the order matching engine
// @Tags Orders
// @Accept json
// @Produce json
// @Param order body models.Order true "Order payload"
// @Success 201 {object} models.Order
// @Failure 400 {object} map[string]string
// @Router /orders [post]
func (h *HTTPHandler) PlaceOrder(c *gin.Context) {
	var req OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order := &models.Order{
		ID:       generateOrderID(),
		Symbol:   req.Symbol,
		Side:     req.Side,
		Price:    req.Price,
		Quantity: req.Quantity,
	}

	if err := h.repo.SaveOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	trades, err := engine.Match(order, h.orderBook, h.repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(trades) > 0 {
		if err := h.repo.SaveTrades(trades); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if err := h.repo.UpdateOrder(order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if order.Quantity > 0 {
		h.orderBook.AddOrder(order)
	}

	c.JSON(http.StatusCreated, OrderResponse{
		Trades:  trades,
		OrderID: order.ID,
		Message: "Order placed successfully",
	})
}

// GetOrderBook godoc
// @Summary Get order book for a symbol
// @Description Returns best bids and asks for the given trading symbol
// @Tags OrderBook
// @Produce json
// @Param symbol path string true "Trading Symbol"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /orderbook/{symbol} [get]
func (h *HTTPHandler) GetOrderBook(c *gin.Context) {
	symbol := c.Param("symbol")
	bestBid, err := h.repo.GetBestBid(symbol)
	if err != nil {
		bestBid = nil
	}
	bestAsk, err := h.repo.GetBestAsk(symbol)
	if err != nil {
		bestAsk = nil
	}
	c.JSON(http.StatusOK, gin.H{
		"best_bid": bestBid,
		"best_ask": bestAsk,
	})
}

// GetOrderByID godoc
// @Summary Get order by ID
// @Description Returns order details based on the given ID
// @Tags Orders
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} models.Order
// @Failure 404 {object} map[string]string
// @Router /orders/{id} [get]
func (h *HTTPHandler) GetOrderByID(c *gin.Context) {
	id := c.Param("id")
	order, err := h.repo.GetOrderByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

// CancelOrder godoc
// @Summary Cancel an order
// @Description Cancels an existing order if not already filled
// @Tags Orders
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /orders/{id} [delete]
func (h *HTTPHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")
	order, err := h.repo.GetOrderByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	delete := h.repo.DeleteOrder(id)
	if delete != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id": id,
		"status":   "CANCELLED",
		"side":     order.Side,
	})

}

func (h *HTTPHandler) RestoreOrderBook(symbol string) {
	buyOrders, _ := h.repo.GetOpenOrders(symbol, "BUY")
	for _, o := range buyOrders {
		h.orderBook.AddOrder(o)
	}

	sellOrders, _ := h.repo.GetOpenOrders(symbol, "SELL")
	for _, o := range sellOrders {
		h.orderBook.AddOrder(o)
	}
}

// Helper Functions

func generateOrderID() string {
	return uuid.New().String()
}
