package main

import (
	"fmt"
	"os"

	api "github.com/Im-Manav/order-matching-engine/internal/api/http"
	"github.com/Im-Manav/order-matching-engine/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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
	orders := r.Group("/orders")
	{
		orders.POST("", h.PlaceOrder)
		orders.GET("/:id", h.GetOrderByID)
		orders.DELETE("/:id", h.CancelOrder)
	}
	r.GET("/orderbook/:symbol", h.GetOrderBook)
	r.Run(os.Getenv("APP_PORT"))
}
