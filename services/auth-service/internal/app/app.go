package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/publisher"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/service"
	grpcserver "github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/transport/grpc"
	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/transport/http"
	authv1 "github.com/Temych228/ap2_protos_go_final/auth/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type App struct {
	cfg *config.Config

	db    *pgxpool.Pool
	cache *redis.Client

	nats *nats.Conn

	grpcServer   *grpc.Server
	grpcListener net.Listener
	httpServer   *http.Server
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

	repo := repository.New(db, cache)
	pub := publisher.New(nc)
	svc := service.New(cfg, repo, pub)

	grpcSrv := grpc.NewServer()
	authv1.RegisterAuthServiceServer(grpcSrv, grpcserver.New(svc))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	httptransport.New(svc).Register(mux)

	httpServer := &http.Server{
		Addr:    cfg.Address(),
		Handler: mux,
	}

	return &App{
		nats:       nc,
		cfg:        cfg,
		db:         db,
		cache:      cache,
		grpcServer: grpcSrv,
		httpServer: httpServer,
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
			log.Printf("grpc server stopped: %v", err)
		}
	}()

	go func() {
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server stopped: %v", err)
		}
	}()

	log.Printf("auth-service started on %s and grpc on :%s", a.cfg.Address(), a.cfg.GRPCPort)
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if a.httpServer != nil {
		_ = a.httpServer.Shutdown(shutdownCtx)
	}

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

	if a.nats != nil {
		a.nats.Close()
	}

	return nil
}
