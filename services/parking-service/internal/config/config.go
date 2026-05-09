package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort      string
	GRPCPort      string
	DBURL         string
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	CacheTTL      time.Duration
}

func LoadConfig() (*Config, error) {
	cacheTTL, err := time.ParseDuration(getEnv("CACHE_TTL", "5m"))
	if err != nil {
		return nil, err
	}

	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "4"))
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPPort:      getEnv("HTTP_PORT", "8085"),
		GRPCPort:      getEnv("GRPC_PORT", "9095"),
		DBURL:         getEnv("DB_URL", "postgres://parking:parking@localhost:5432/parking_service?sslmode=disable"),
		RedisHost:     getEnv("REDIS_HOST", "redis"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,
		CacheTTL:      cacheTTL,
	}, nil
}

func getEnv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func (c *Config) HTTPAddress() string { return fmt.Sprintf(":%s", c.HTTPPort) }
func (c *Config) GRPCAddress() string { return fmt.Sprintf(":%s", c.GRPCPort) }
func (c *Config) RedisAddr() string   { return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort) }
