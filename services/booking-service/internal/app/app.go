package app

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/client"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/publisher"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/service"
	grpcserver "github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/transport/grpc"

	bookingv1 "github.com/Temych228/ap2_protos_go_final/booking/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	cfg *config.Config

	db    *pgxpool.Pool
	cache *redis.Client
	nats  *nats.Conn

	userConn    *grpc.ClientConn
	parkingConn *grpc.ClientConn

	grpcServer   *grpc.Server
	grpcListener net.Listener
}

func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	db, err := pgxpool.New(ctx, cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}

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

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userConn, err := grpc.DialContext(
		dialCtx,
		cfg.UserGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		nc.Close()
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	parkingConn, err := grpc.DialContext(
		dialCtx,
		cfg.ParkingGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		_ = userConn.Close()
		nc.Close()
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	repo := repository.NewBookingRepository(db, cache, cfg.CacheTTL)
	svc := service.New(
		repo,
		client.NewUserClient(userConn),
		client.NewParkingClient(parkingConn),
		publisher.New(nc),
	)

	grpcSrv := grpc.NewServer()
	bookingv1.RegisterBookingServiceServer(grpcSrv, grpcserver.New(svc))

	return &App{
		cfg:         cfg,
		db:          db,
		cache:       cache,
		nats:        nc,
		userConn:    userConn,
		parkingConn: parkingConn,
		grpcServer:  grpcSrv,
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

	log.Printf("booking-service started on grpc :%s", a.cfg.GRPCPort)
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = shutdownCtx

	if a.grpcServer != nil {
		a.grpcServer.GracefulStop()
	}
	if a.grpcListener != nil {
		_ = a.grpcListener.Close()
	}
	if a.parkingConn != nil {
		_ = a.parkingConn.Close()
	}
	if a.userConn != nil {
		_ = a.userConn.Close()
	}
	if a.nats != nil {
		a.nats.Close()
	}
	if a.cache != nil {
		_ = a.cache.Close()
	}
	if a.db != nil {
		a.db.Close()
	}
	return nil
}
