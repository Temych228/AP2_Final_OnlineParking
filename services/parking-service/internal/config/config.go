package config

import "os"

type Config struct {
	DBUrl    string
	GRPCPort string
}

func LoadConfig() Config {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/parking_db?sslmode=disable"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}

	return Config{
		DBUrl:    dbURL,
		GRPCPort: grpcPort,
	}
}
