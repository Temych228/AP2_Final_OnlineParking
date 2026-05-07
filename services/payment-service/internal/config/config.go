package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort string
	GRPCPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	BookingServiceURL string
	ParkingServiceURL string
	NATSURL           string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		HTTPPort: getEnv("HTTP_PORT", "8086"),
		GRPCPort: getEnv("GRPC_PORT", "9096"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "payment_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		BookingServiceURL: getEnv("BOOKING_SERVICE_URL", "http://localhost:8084"),
		ParkingServiceURL: getEnv("PARKING_SERVICE_URL", "http://localhost:8085"),
		NATSURL:           getEnv("NATS_URL", "nats://localhost:4222"),
	}

	return cfg, nil
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost,
		c.DBPort,
		c.DBUser,
		c.DBPassword,
		c.DBName,
		c.DBSSLMode,
	)
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
