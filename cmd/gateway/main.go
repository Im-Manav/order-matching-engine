package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Im-Manav/ome/internal/api"
	"github.com/Im-Manav/ome/internal/api/ws"
	"github.com/Im-Manav/ome/internal/cache"
	"github.com/Im-Manav/ome/internal/config"
	"github.com/Im-Manav/ome/internal/db"
	"github.com/Im-Manav/ome/internal/kafka"
	"github.com/Im-Manav/ome/internal/service"
	"github.com/Im-Manav/ome/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// ── Logger ────────────────────────────────────────────────────────────────
	if err := logger.Init(cfg.Env); err != nil {
		panic("failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	database, err := db.NewConnection(cfg)
	if err != nil {
		logger.Fatal("postgres connection failed", logger.Err(err))
	}
	if err := db.Migrate(database); err != nil {
		logger.Fatal("migration failed", logger.Err(err))
	}
	logger.Info("postgres connected and migrated")

	// Redis
	redisClient, err := cache.NewClient(cfg)
	if err != nil {
		logger.Fatal("redis connection failed", logger.Err(err))
	}
	defer redisClient.Close()
	logger.Info("redis connected")

	// Kafka producer
	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()
	logger.Info("kafka producer ready")

	// Repository
	repo := db.NewRepository(database)

	// Websocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// Services
	authSvc := service.NewAuthService(repo, redisClient, cfg)
	orderSvc := service.NewOrderService(repo, repo, producer, redisClient, hub)

	// Gin
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	handler := api.NewHandler(orderSvc, authSvc, hub, redisClient)
	handler.RegisterRoutes(r)

	srv := &http.Server{
		Addr:         ":" + cfg.GatewayPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	go func() {
		logger.Info("gateway starting", zap.String("port", cfg.GatewayPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("gateway failed", logger.Err(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	// Wait for SIGINT or SIGTERM (sent by K8s on pod termination)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received")

	// Give in-flight requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("gateway forced shutdown", logger.Err(err))
	}

	logger.Info("gateway stopped cleanly")

}

// requestLogger returns a Gin middleware that logs every request
// with method, path, status, and latency using zap.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
		)
	}
}
