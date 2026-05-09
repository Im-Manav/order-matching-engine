package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Env            string
	GatewayPort    string
	EnginePort     string
	MarketDataPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	RedisHost     string
	RedisPort     string
	RedisPassword string

	KafkaBrokers []string
	KafkaGroupID string

	JWTSecret      string
	JWTExpiryHours int
}

func Load() (*Config, error) {
	// In production the env vars are injected by K8s — .env is for local dev only
	_ = godotenv.Load()

	jwtExpiry, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	if err != nil {
		jwtExpiry = 24
	}

	cfg := &Config{
		Env:            getEnv("ENV", "development"),
		GatewayPort:    getEnv("GATEWAY_PORT", "8080"),
		EnginePort:     getEnv("ENGINE_PORT", "8081"),
		MarketDataPort: getEnv("MARKET_DATA_PORT", "8082"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "ome"),
		DBPassword: getEnv("DB_PASSWORD", "ome_secret"),
		DBName:     getEnv("DB_NAME", "ome_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		KafkaBrokers: strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "ome-engine"),

		JWTSecret:      getEnv("JWT_SECRET", "change_me"),
		JWTExpiryHours: jwtExpiry,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.JWTSecret == "change_me" && c.Env == "Production" {
		return fmt.Errorf("JWT_SECRET must be set in production")
	}
	return nil
}

func (c *Config) DBConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
