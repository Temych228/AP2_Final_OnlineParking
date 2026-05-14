package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/service"
	grpcserver "github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/transport/grpc"
	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/transport/http"
	notificationv1 "github.com/Temych228/ap2_protos_go_final/notification/v1"
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
	nats  *nats.Conn

	grpcServer   *grpc.Server
	grpcListener net.Listener

	httpServer    *http.Server
	metricsServer *http.Server

	svc *service.NotificationService
}

func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
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

	nc, err := nats.Connect(cfg.NATSUrl)
	if err != nil {
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	repo := repository.New(db, cache)
	hub := service.NewHub()
	svc := service.New(cfg, repo, hub)

	grpcSrv := grpc.NewServer()
	notificationv1.RegisterNotificationServiceServer(grpcSrv, grpcserver.New(svc))

	httpMux := http.NewServeMux()
	httptransport.New(svc).Register(httpMux)

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddress(),
		Handler: httpMux,
	}

	metricsServer := &http.Server{
		Addr:    cfg.MetricsAddress(),
		Handler: promhttp.Handler(),
	}

	app := &App{
		cfg:           cfg,
		db:            db,
		cache:         cache,
		nats:          nc,
		grpcServer:    grpcSrv,
		httpServer:    httpServer,
		metricsServer: metricsServer,
		svc:           svc,
	}

	if err := app.subscribeNATS(); err != nil {
		nc.Close()
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	return app, nil
}

func (a *App) subscribeNATS() error {
	subjects := []string{
		"parking.user.registered",
		"parking.payment.success",
		"parking.booking.confirmed",
		"parking.booking.cancelled",
		"parking.auth.password_reset",
	}

	for _, subject := range subjects {
		if _, err := a.nats.Subscribe(subject, func(msg *nats.Msg) {
			if err := a.svc.HandleEvent(context.Background(), msg.Subject, msg.Data); err != nil {
				log.Printf("nats handler error subject=%s err=%v", msg.Subject, err)
			}
		}); err != nil {
			return err
		}
	}

	return a.nats.Flush()
}

func (a *App) Run(ctx context.Context) error {
	grpcLis, err := net.Listen("tcp", a.cfg.GRPCAddress())
	if err != nil {
		return err
	}
	a.grpcListener = grpcLis

	httpLis, err := net.Listen("tcp", a.cfg.HTTPAddress())
	if err != nil {
		_ = grpcLis.Close()
		return err
	}

	metricsLis, err := net.Listen("tcp", a.cfg.MetricsAddress())
	if err != nil {
		_ = grpcLis.Close()
		_ = httpLis.Close()
		return err
	}

	go func() {
		if err := a.grpcServer.Serve(grpcLis); err != nil {
			log.Printf("grpc server stopped: %v", err)
		}
	}()

	go func() {
		if err := a.httpServer.Serve(httpLis); err != nil && err != http.ErrServerClosed {
			log.Printf("http server stopped: %v", err)
		}
	}()

	go func() {
		if err := a.metricsServer.Serve(metricsLis); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server stopped: %v", err)
		}
	}()

	log.Printf("notification-service started on %s, grpc on %s, metrics on %s",
		a.cfg.HTTPAddress(), a.cfg.GRPCAddress(), a.cfg.MetricsAddress())
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if a.httpServer != nil {
		_ = a.httpServer.Shutdown(shutdownCtx)
	}
	if a.metricsServer != nil {
		_ = a.metricsServer.Shutdown(shutdownCtx)
	}
	if a.grpcServer != nil {
		a.grpcServer.GracefulStop()
	}
	if a.grpcListener != nil {
		_ = a.grpcListener.Close()
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
