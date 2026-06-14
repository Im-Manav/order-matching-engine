package api

import (
	"net/http"

	"github.com/Im-Manav/ome/internal/api/ws"
	"github.com/Im-Manav/ome/internal/ports"
	"github.com/Im-Manav/ome/internal/service"
	apperrors "github.com/Im-Manav/ome/pkg/errors"
	"github.com/Im-Manav/ome/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// OrderService is the interface the handler needs.
// Defined here so we can mock it in tests without
// importing the full service package.
type OrderService interface {
	PlaceOrder(ctx interface{}, req models.PlaceOrderRequest, userID uuid.UUID) (*models.PlaceOrderResponse, error)
	CancelOrder(ctx interface{}, orderID uuid.UUID, userID uuid.UUID) error
	GetOrderBook(ctx interface{}, symbol string) (*models.OrderBookSnapshot, error)
	GetUserOrders(ctx interface{}, userID uuid.UUID) ([]*models.Order, error)
	GetRecentTrades(ctx interface{}, symbol string, limit int) ([]models.Trade, error)
}

// Handler holds all dependencies for HTTP handlers.
type Handler struct {
	orderSvc *service.OrderService
	authSvc  *service.AuthService
	hub      *ws.Hub
	cache    ports.Cache
}

func NewHandler(
	orderSvc *service.OrderService,
	authSvc *service.AuthService,
	hub *ws.Hub,
	cache ports.Cache,
) *Handler {
	return &Handler{
		orderSvc: orderSvc,
		authSvc:  authSvc,
		hub:      hub,
		cache:    cache,
	}
}

// RegisterRoutes wires all routes onto the Gin engine.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// Health check — no auth required
	r.GET("/health", h.Health)

	// WebSocket — no auth (token passed as query param from browser)
	r.GET("/ws", h.WebSocket)

	// Auth routes — no auth middleware
	auth := r.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/logout", h.Logout)
	}

	// Protected routes — JWT required
	api := r.Group("/api/v1")
	api.Use(Auth(h.authSvc))
	api.Use(RateLimit(h.cache))
	{
		// Orders
		orders := api.Group("/orders")
		{
			orders.POST("", h.PlaceOrder)
			orders.GET("", h.GetUserOrders)
			orders.DELETE("/:id", h.CancelOrder)
		}

		// Order book + trades — read-only, still auth-protected
		api.GET("/orderbook/:symbol", h.GetOrderBook)
		api.GET("/trades/:symbol", h.GetRecentTrades)
	}
}

// ─── Health ───────────────────────────────────────────────────────────────────

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"clients": h.hub.ClientCount(),
	})
}

// ─── WebSocket ────────────────────────────────────────────────────────────────

func (h *Handler) WebSocket(c *gin.Context) {
	symbol := c.Query("symbol") // e.g. /ws?symbol=BTC-USD
	h.hub.ServeWS(c.Writer, c.Request, symbol)
}

// ─── Auth handlers ────────────────────────────────────────────────────────────

func (h *Handler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authSvc.Register(c.Request.Context(), req)
	if err != nil {
		appErr := apperrors.ToHTTP(err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authSvc.Login(c.Request.Context(), req)
	if err != nil {
		// Always 401 for auth failures — never reveal which field was wrong
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Logout(c *gin.Context) {
	// Get token from Authorization header
	tokenString := extractToken(c)
	if tokenString == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no token provided"})
		return
	}

	if err := h.authSvc.Logout(c.Request.Context(), tokenString); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// ─── Order handlers ───────────────────────────────────────────────────────────

func (h *Handler) PlaceOrder(c *gin.Context) {
	var req models.PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := mustGetUserID(c)

	resp, err := h.orderSvc.PlaceOrder(c.Request.Context(), req, userID)
	if err != nil {
		appErr := apperrors.ToHTTP(err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) CancelOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	userID := mustGetUserID(c)

	if err := h.orderSvc.CancelOrder(c.Request.Context(), orderID, userID); err != nil {
		appErr := apperrors.ToHTTP(err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, models.CancelOrderResponse{
		OrderID: orderID.String(),
		Status:  models.StatusCancelled.String(),
	})
}

func (h *Handler) GetUserOrders(c *gin.Context) {
	userID := mustGetUserID(c)

	orders, err := h.orderSvc.GetUserOrders(c.Request.Context(), userID)
	if err != nil {
		appErr := apperrors.ToHTTP(err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

func (h *Handler) GetOrderBook(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol required"})
		return
	}

	snap, err := h.orderSvc.GetOrderBook(c.Request.Context(), symbol)
	if err != nil {
		appErr := apperrors.ToHTTP(err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, snap)
}

func (h *Handler) GetRecentTrades(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol required"})
		return
	}

	trades, err := h.orderSvc.GetRecentTrades(c.Request.Context(), symbol, 50)
	if err != nil {
		appErr := apperrors.ToHTTP(err)
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"trades": trades})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// mustGetUserID extracts the userID injected by the Auth middleware.
// Panics if not present — which means a route is missing the Auth middleware.
func mustGetUserID(c *gin.Context) uuid.UUID {
	val, _ := c.Get(ContextUserID)
	return val.(uuid.UUID)
}

func extractToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if len(header) > 7 && header[:7] == "Bearer " {
		return header[7:]
	}
	return ""
}
