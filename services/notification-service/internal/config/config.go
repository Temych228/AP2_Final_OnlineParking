package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPPort    string
	GRPCPort    string
	MetricsPort string

	DatabaseURL string

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	NATSUrl    string
	NumWorkers int

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	FrontendURL string
}

func Load() (*Config, error) {
	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "2"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT: %w", err)
	}

	numWorkers, err := strconv.Atoi(getEnv("NATS_NUM_WORKERS", "5"))
	if err != nil || numWorkers <= 0 {
		numWorkers = 5
	}

	return &Config{
		HTTPPort:      getEnv("HTTP_PORT", "8083"),
		GRPCPort:      getEnv("GRPC_PORT", "9093"),
		MetricsPort:   getEnv("METRICS_PORT", "9203"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,
		NATSUrl:       getEnv("NATS_URL", "nats://localhost:4222"),
		NumWorkers:    numWorkers,
		SMTPHost:      getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:      smtpPort,
		SMTPUsername:  getEnv("SMTP_USERNAME", ""),
		SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:      getEnv("SMTP_FROM", ""),
		FrontendURL:   getEnv("FRONTEND_URL", "http://localhost:3000"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c *Config) HTTPAddress() string    { return fmt.Sprintf(":%s", c.HTTPPort) }
func (c *Config) GRPCAddress() string    { return fmt.Sprintf(":%s", c.GRPCPort) }
func (c *Config) MetricsAddress() string { return fmt.Sprintf(":%s", c.MetricsPort) }
func (c *Config) RedisAddr() string      { return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort) }
