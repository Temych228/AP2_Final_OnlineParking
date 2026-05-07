package main

import (
	"log"

	"payment-service/internal/app"
	"payment-service/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	application := app.New(cfg)

	if err := application.Run(); err != nil {
		log.Fatalf("payment-service stopped with error: %v", err)
	}
}
