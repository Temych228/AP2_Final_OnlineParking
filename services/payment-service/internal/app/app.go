package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"

	"payment-service/internal/config"
	httpdelivery "payment-service/internal/delivery/http"
	"payment-service/internal/integration"
	"payment-service/internal/publisher"
	"payment-service/internal/repository"
	"payment-service/internal/service"
)

type App struct {
	cfg   *config.Config
	db    *sql.DB
	nc    *nats.Conn
	cache *redis.Client
}

func New(cfg *config.Config) *App {
	return &App{
		cfg: cfg,
	}
}

func (a *App) Run() error {
	if err := a.connectDB(); err != nil {
		return err
	}
	defer a.db.Close()

	if err := a.runMigrations(); err != nil {
		return err
	}

	if err := a.connectNATS(); err != nil {
		return err
	}
	defer a.nc.Close()

	if err := a.connectRedis(); err != nil {
		return err
	}
	defer a.cache.Close()

	paymentRepo := repository.NewPaymentRepository(a.db)
	bookingIntegration := integration.NewBookingIntegration(a.cfg.BookingServiceURL)
	parkingIntegration := integration.NewParkingIntegration(a.cfg.ParkingServiceURL)
	natsPublisher := publisher.NewNATSPublisher(a.nc)

	paymentService := service.NewPaymentService(
		paymentRepo,
		bookingIntegration,
		parkingIntegration,
		natsPublisher,
	)
	paymentService.SetCache(a.cache)

	router := gin.Default()
	paymentHandler := httpdelivery.NewPaymentHandler(paymentService)
	paymentHandler.RegisterRoutes(router)

	addr := a.cfg.HTTPAddress()

	log.Printf("payment-service HTTP server is running on %s", addr)

	return router.Run(addr)
}

func (a *App) connectDB() error {
	db, err := sql.Open("postgres", a.cfg.DatabaseURL())
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	a.db = db

	log.Println("connected to PostgreSQL")

	return nil
}

func (a *App) connectNATS() error {
	nc, err := nats.Connect(a.cfg.NATSURL)
	if err != nil {
		return err
	}

	a.nc = nc

	log.Println("connected to NATS")

	return nil
}

func (a *App) connectRedis() error {
	cache := redis.NewClient(&redis.Options{
		Addr:     a.cfg.RedisAddr(),
		Password: a.cfg.RedisPassword,
		DB:       a.cfg.RedisDB,
	})

	if err := cache.Ping(context.Background()).Err(); err != nil {
		return err
	}

	a.cache = cache

	log.Println("connected to Redis")

	return nil
}

func (a *App) runMigrations() error {
	path := "migrations/001_create_payments_table.sql"

	migration, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	if _, err := a.db.Exec(string(migration)); err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	log.Println("payment-service migrations applied")

	return nil
}
