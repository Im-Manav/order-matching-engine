// @title Order Matching Engine API
// @version 1.0
// @description REST API for placing, cancelling, and viewing orders & order book.
// @BasePath /

// @contact.name Manav Gupta
// @contact.email your-email@example.com

// @host localhost:8080
// @schemes http
package main

import (
	"fmt"
	"os"

	"github.com/Im-Manav/order-matching-engine/internal/api/docs"
	api "github.com/Im-Manav/order-matching-engine/internal/api/http"
	"github.com/Im-Manav/order-matching-engine/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	godotenv.Load()
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	if host == "" || user == "" || password == "" || dbname == "" || port == "" {
		panic("Missing required database environment variables")
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", host, user, password, dbname, port)
	gormDB := db.NewGormDB(dsn)
	repo := db.NewRepo(gormDB)

	h := api.NewHTTPHandler(repo)
	r := gin.Default()

	// Swagger setup
	docs.SwaggerInfo.Title = "Order Matching Engine API"
	docs.SwaggerInfo.Description = "API documentation for Order Matching Engine"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.BasePath = "/"
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	orders := r.Group("/orders")
	{
		orders.POST("", h.PlaceOrder)
		orders.GET("/:id", h.GetOrderByID)
		orders.DELETE("/:id", h.CancelOrder)
	}
	r.GET("/orderbook/:symbol", h.GetOrderBook)
	r.Run(":" + os.Getenv("APP_PORT"))
}

// This is from the test branch
