package app

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/service"
	grpcserver "github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/transport/grpc"
	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/transport/http"
	"github.com/nats-io/nats.go"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	userv1 "github.com/Temych228/ap2_protos_go_final/user/v1"
)

type App struct {
	cfg *config.Config

	db    *pgxpool.Pool
	cache *redis.Client

	grpcServer   *grpc.Server
	grpcListener net.Listener

	nats *nats.Conn

	httpServer    *http.Server
	metricsServer *http.Server
}

type userRegisteredEvent struct {
	EventID           string `json:"event_id"`
	UserID            string `json:"user_id"`
	UserEmail         string `json:"user_email"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Phone             string `json:"phone"`
	VerificationToken string `json:"verification_token"`
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

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	if err := cache.Ping(ctx).Err(); err != nil {
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	repo := repository.NewUserRepository(db, cache, cfg.CacheTTL)
	svc := service.New(repo)

	if err := subscribeUserRegistered(nc, svc); err != nil {
		nc.Close()
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	grpcSrv := grpc.NewServer()
	userv1.RegisterUserServiceServer(grpcSrv, grpcserver.New(svc))

	router := gin.New()
	router.Use(gin.Recovery())

	httpHandler := httptransport.New()
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
		grpcServer:    grpcSrv,
		httpServer:    httpServer,
		metricsServer: metricsServer,
		nats:          nc,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	grpcLis, err := net.Listen("tcp", a.cfg.GRPCAddress())
	if err != nil {
		return err
	}
	a.grpcListener = grpcLis

	httpLis, err := net.Listen("tcp", a.cfg.Address())
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

	log.Printf("user-service started on %s, grpc on %s, metrics on %s", a.cfg.Address(), a.cfg.GRPCAddress(), a.cfg.MetricsAddress())
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

func subscribeUserRegistered(nc *nats.Conn, svc *service.UserService) error {
	_, err := nc.Subscribe("parking.user.registered", func(msg *nats.Msg) {
		var ev userRegisteredEvent
		if err := json.Unmarshal(msg.Data, &ev); err != nil {
			log.Printf("user-service nats unmarshal error: %v", err)
			return
		}

		_, err := svc.CreateUserWithID(context.Background(), ev.UserID, domain.CreateInput{
			Email:     ev.UserEmail,
			FirstName: ev.FirstName,
			LastName:  ev.LastName,
			Phone:     ev.Phone,
			Role:      domain.RoleUser,
		})
		if err != nil {
			log.Printf("user-service create profile error: %v", err)
		}
	})
	if err != nil {
		return err
	}
	return nc.Flush()
}
