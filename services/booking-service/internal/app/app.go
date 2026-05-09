package app

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/service"
	grpcserver "github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/transport/grpc"

	bookingv1 "github.com/Temych228/ap2_protos_go_final/booking/v1"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type App struct {
	cfg *config.Config

	db    *pgxpool.Pool
	cache *redis.Client

	grpcServer   *grpc.Server
	grpcListener net.Listener
}

func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	// DB
	db, err := pgxpool.New(ctx, cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}

	// Redis
	cache := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	if err := cache.Ping(ctx).Err(); err != nil {
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	// Repo → Service
	repo := repository.NewBookingRepository(db, cache, cfg.CacheTTL)
	svc := service.New(repo)

	// gRPC server
	grpcSrv := grpc.NewServer()
	bookingv1.RegisterBookingServiceServer(
		grpcSrv,
		grpcserver.New(svc),
	)

	return &App{
		cfg:        cfg,
		db:         db,
		cache:      cache,
		grpcServer: grpcSrv,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	grpcLis, err := net.Listen("tcp", ":"+a.cfg.GRPCPort)
	if err != nil {
		return err
	}
	a.grpcListener = grpcLis

	go func() {
		if err := a.grpcServer.Serve(grpcLis); err != nil {
			log.Printf("booking grpc stopped: %v", err)
		}
	}()

	log.Printf(
		"booking-service started on grpc :%s",
		a.cfg.GRPCPort,
	)

	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if a.grpcServer != nil {
		a.grpcServer.GracefulStop()
	}

	if a.grpcListener != nil {
		_ = a.grpcListener.Close()
	}

	if a.cache != nil {
		_ = a.cache.Close()
	}

	if a.db != nil {
		a.db.Close()
	}

	_ = shutdownCtx

	return nil
}
