package app

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	paymentv1 "github.com/Temych228/ap2_protos_go_final/payment/v1"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"

	"payment-service/internal/config"
	grpcdelivery "payment-service/internal/delivery/grpc"
	httpdelivery "payment-service/internal/delivery/http"
	"payment-service/internal/integration"
	"payment-service/internal/publisher"
	"payment-service/internal/repository"
	"payment-service/internal/service"
)

type App struct {
	cfg *config.Config
	db  *sql.DB
	nc  *nats.Conn
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

	if err := a.startGRPCServer(paymentService); err != nil {
		return err
	}

	router := gin.Default()

	paymentHandler := httpdelivery.NewPaymentHandler(paymentService)
	paymentHandler.RegisterRoutes(router)

	addr := ":" + a.cfg.HTTPPort

	log.Printf("payment-service HTTP server is running on %s", addr)

	return router.Run(addr)
}

func (a *App) startGRPCServer(paymentService *service.PaymentService) error {
	grpcAddr := ":" + a.cfg.GRPCPort

	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC address %s: %w", grpcAddr, err)
	}

	grpcServer := grpc.NewServer()

	paymentv1.RegisterPaymentServiceServer(
		grpcServer,
		grpcdelivery.New(paymentService),
	)

	go func() {
		log.Printf("payment-service gRPC server is running on %s", grpcAddr)

		if err := grpcServer.Serve(listener); err != nil {
			log.Printf("payment-service gRPC server stopped: %v", err)
		}
	}()

	return nil
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
