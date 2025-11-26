package http

import (
	"net/http"

	"github.com/Im-Manav/order-matching-engine/internal/engine"
	"github.com/Im-Manav/order-matching-engine/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type HTTPHandler struct {
	orderBook *engine.OrderBook
}

func NewHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		orderBook: engine.NewOrderBook(),
	}
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

	trades, err := engine.Match(order, h.orderBook)
	if err != nil {
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

func (h *HTTPHandler) GetOrderBook(c *gin.Context) {
	ob := h.orderBook
	bestBid := ob.GetBestBid()
	bestAsk := ob.GetBestAsk()

	c.JSON(http.StatusOK, gin.H{
		"best_bid": bestBid,
		"best_ask": bestAsk,
	})
}

func (h *HTTPHandler) GetOrderByID(c *gin.Context) {
	id := c.Param("id")
	order := h.orderBook.FindOrder(id)
	if order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *HTTPHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")
	found, side := h.orderBook.Cancel(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id": id,
		"status":   "CANCELLED",
		"side":     side,
	})

}

// Helper Functions

func generateOrderID() string {
	return uuid.New().String()
}
