package config

import (
	"fmt"
	"os"
	"strconv"

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

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	BookingServiceURL string
	ParkingServiceURL string
	UserServiceURL    string
	NATSURL           string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "5"))
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPPort: getEnv("HTTP_PORT", "8086"),
		GRPCPort: getEnv("GRPC_PORT", "9096"),

		DBHost:     getEnv("DB_HOST", "postgres"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "parking"),
		DBPassword: getEnv("DB_PASSWORD", "parking"),
		DBName:     getEnv("DB_NAME", "payment_service"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		RedisHost:     getEnv("REDIS_HOST", "redis"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		BookingServiceURL: getEnv("BOOKING_SERVICE_URL", "http://localhost:8084"),
		ParkingServiceURL: getEnv("PARKING_SERVICE_URL", "http://localhost:8085"),
		UserServiceURL:    getEnv("USER_SERVICE_URL", "http://localhost:8081"),
		NATSURL:           getEnv("NATS_URL", "nats://localhost:4222"),
	}, nil
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

func (c *Config) HTTPAddress() string {
	return fmt.Sprintf(":%s", c.HTTPPort)
}

func (c *Config) GRPCAddress() string {
	return fmt.Sprintf(":%s", c.GRPCPort)
}

func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
