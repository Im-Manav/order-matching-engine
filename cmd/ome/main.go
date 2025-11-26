package main

import (
	api "github.com/Im-Manav/order-matching-engine/internal/api/http"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	h := api.NewHTTPHandler()
	orders := r.Group("/orders")
	{
		orders.POST("", h.PlaceOrder)
		orders.GET("/:id", h.GetOrderByID)
		orders.DELETE("/:id", h.CancelOrder)
	}
	r.GET("/orderbook", h.GetOrderBook)
	r.Run(":8080")
}
