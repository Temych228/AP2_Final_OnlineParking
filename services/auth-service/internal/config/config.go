package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppPort  string
	GRPCPort string

	DatabaseURL string

	NATSURL string

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	JWTSecret        string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	VerificationTTL  time.Duration
	PasswordResetTTL time.Duration
}

func Load() (*Config, error) {
	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "1"))
	if err != nil {
		return nil, err
	}

	accessTTL, err := time.ParseDuration(getEnv("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return nil, err
	}

	refreshTTL, err := time.ParseDuration(getEnv("REFRESH_TOKEN_TTL", "720h"))
	if err != nil {
		return nil, err
	}

	verificationTTL, err := time.ParseDuration(getEnv("VERIFICATION_TOKEN_TTL", "24h"))
	if err != nil {
		return nil, err
	}

	resetTTL, err := time.ParseDuration(getEnv("PASSWORD_RESET_TTL", "1h"))
	if err != nil {
		return nil, err
	}

	return &Config{
		AppPort:          getEnv("APP_PORT", "8082"),
		GRPCPort:         getEnv("GRPC_PORT", "9092"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		RedisHost:        getEnv("REDIS_HOST", "localhost"),
		RedisPort:        getEnv("REDIS_PORT", "6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          redisDB,
		JWTSecret:        getEnv("JWT_SECRET", "change-me"),
		NATSURL:          getEnv("NATS_URL", "nats://nats:4222"),
		AccessTokenTTL:   accessTTL,
		RefreshTokenTTL:  refreshTTL,
		VerificationTTL:  verificationTTL,
		PasswordResetTTL: resetTTL,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c *Config) Address() string {
	return fmt.Sprintf(":%s", c.AppPort)
}

func (c *Config) PostgresDSN() string {
	return c.DatabaseURL
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}
