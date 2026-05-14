package main

import (
	"log"

	_ "github.com/lib/pq"

	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/app"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/config"
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
