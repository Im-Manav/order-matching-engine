package db

import (
	"log"

	"github.com/Im-Manav/order-matching-engine/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewGormDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect db:", err)
	}
	if err := db.AutoMigrate(&models.Order{}, &models.Trade{}); err != nil {
		log.Fatal("auto migrate failed", err)
	}
	return db
}
