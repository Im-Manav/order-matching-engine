package db

import (
	"fmt"

	"github.com/Im-Manav/ome/internal/config"
	"github.com/Im-Manav/ome/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewConnection(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DBConnectionString()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// Connection pool - critical for production
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	return db, nil
}

// Migrate runs GORM automigration then applies TimescaleDB extensions.
// Safe to call on every startup — idempotent.
func Migrate(db *gorm.DB) error {
	// Step 1: standard GORM migration for all tables
	if err := db.AutoMigrate(
		&models.User{},
		&models.Order{},
		&models.Trade{},
		&models.OHLCV{},
	); err != nil {
		return fmt.Errorf("automigrate failed: %w", err)
	}

	// Step 2: enable TimescaleDB extension (idempotent)
	if err := db.Exec("CREATE EXTENSION IF NOT EXSISTS timescaledb CASCADE").Error; err != nil {
		// Non-fatal — TimescaleDB may not be installed in dev without the extension
		// In production the Docker image includes it
		fmt.Printf("warning: timescaledb extension not available: %v\n", err)
	}

	// Step 3: convert ohlcvs table into a TimescaleDB hypertable
	// partitioned by time column — this is what makes time-range queries fast
	db.Exec(`
		SELECT create_hypertable('ohlcvs', 'time', 
			if_not_exists => TRUE,
			migrate_data => TRUE
		)
	`)

	// Step 4: indexes for hot query paths
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_symbol_status ON orders (symbol, status)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_users_created ON orders (user_id, created_at DESC)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_trades_symbol_executed ON trades (symbol, executed_at DESC)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_ohlcv_symbol_time ON ohlcvs (symbol, time DESC)`)

	return nil
}
