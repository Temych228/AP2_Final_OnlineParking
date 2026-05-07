package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/transport/http"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type App struct {
	cfg *config.Config

	db    *pgxpool.Pool
	cache *redis.Client

	httpServer    *http.Server
	metricsServer *http.Server
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func ensureDatabase(ctx context.Context, cfg *config.Config) error {
	if cfg.DatabaseURL != "" || cfg.DBName == "postgres" {
		return nil
	}

	// Use the configured user (which should be the superuser) for database creation
	maintenanceDSN := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/postgres?sslmode=%s",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBSSLMode,
	)

	conn, err := pgx.Connect(ctx, maintenanceDSN)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Ensure the user has database creation privileges
	_, err = conn.Exec(ctx, fmt.Sprintf("ALTER USER %s CREATEDB", quoteIdentifier(cfg.DBUser)))
	if err != nil {
		// Ignore error if already has privileges
		log.Printf("Warning: could not grant CREATEDB to user: %v", err)
	}

	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.DBName).Scan(&exists); err != nil {
		return err
	}

	if exists {
		return nil
	}

	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(cfg.DBName)))
	if err != nil {
		return err
	}

	// Grant privileges to the application user
	_, err = conn.Exec(ctx, fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", quoteIdentifier(cfg.DBName), quoteIdentifier(cfg.DBUser)))
	return err
}

func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	if err := ensureDatabase(ctx, cfg); err != nil {
		return nil, err
	}

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

	repo := repository.NewBookingRepository(db, cache, cfg.CacheTTL)
	svc := service.New(repo)

	router := gin.New()
	router.Use(gin.Recovery())

	httpHandler := httptransport.New(svc)
	httpHandler.Register(router)

	httpServer := &http.Server{
		Addr:    cfg.Address(),
		Handler: router,
	}

	metricsServer := &http.Server{
		Addr:    cfg.MetricsAddress(),
		Handler: promhttp.Handler(),
	}

	return &App{
		cfg:           cfg,
		db:            db,
		cache:         cache,
		httpServer:    httpServer,
		metricsServer: metricsServer,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	httpLis, err := net.Listen("tcp", a.cfg.Address())
	if err != nil {
		return err
	}

	metricsLis, err := net.Listen("tcp", a.cfg.MetricsAddress())
	if err != nil {
		_ = httpLis.Close()
		return err
	}

	go func() {
		<-ctx.Done()
		_ = a.Shutdown(context.Background())
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

	log.Printf("booking-service started on %s, metrics on %s", a.cfg.Address(), a.cfg.MetricsAddress())
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

	if a.cache != nil {
		_ = a.cache.Close()
	}

	if a.db != nil {
		a.db.Close()
	}

	return nil
}
