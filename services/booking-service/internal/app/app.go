package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/client"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/publisher"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/service"
	grpcserver "github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/transport/grpc"
	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/transport/http"

	bookingv1 "github.com/Temych228/ap2_protos_go_final/booking/v1"
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

	httpServer *http.Server
}

func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	// --- Databases ---
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

	// --- Brokers ---
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	// --- gRPC Clients (Dialing other services) ---
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

	// --- Business Logic ---
	repo := repository.NewBookingRepository(db, cache, cfg.CacheTTL)
	svc := service.New(
		repo,
		client.NewUserClient(userConn),
		client.NewParkingClient(parkingConn),
		publisher.New(nc),
	)

	// --- gRPC Server Registration ---
	grpcSrv := grpc.NewServer()
	bookingv1.RegisterBookingServiceServer(grpcSrv, grpcserver.New(svc))

	// --- HTTP/REST Server Setup (Gin) ---
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	httpHandler := httptransport.New(svc)
	httpHandler.Register(router)

	httpSrv := &http.Server{
		Addr:    cfg.Address(), // Использует AppPort (8084)
		Handler: router,
	}

	return &App{
		cfg:         cfg,
		db:          db,
		cache:       cache,
		nats:        nc,
		userConn:    userConn,
		parkingConn: parkingConn,
		grpcServer:  grpcSrv,
		httpServer:  httpSrv,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	// Start gRPC listener
	grpcLis, err := net.Listen("tcp", ":"+a.cfg.GRPCPort)
	if err != nil {
		return err
	}
	a.grpcListener = grpcLis

	// Start gRPC Server
	go func() {
		log.Printf("booking grpc started on :%s", a.cfg.GRPCPort)
		if err := a.grpcServer.Serve(grpcLis); err != nil && err != grpc.ErrServerStopped {
			log.Printf("grpc stopped: %v", err)
		}
	}()

	// Start HTTP/REST Server
	go func() {
		log.Printf("booking http started on %s", a.cfg.Address())
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http stopped: %v", err)
		}
	}()

	// Graceful shutdown watcher
	go func() {
		<-ctx.Done()
		log.Println("received shutdown signal, stopping servers...")
		_ = a.Shutdown(context.Background())
	}()

	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 1. Stop HTTP
	if a.httpServer != nil {
		log.Println("stopping http server...")
		_ = a.httpServer.Shutdown(shutdownCtx)
	}

	// 2. Stop gRPC Server
	if a.grpcServer != nil {
		log.Println("stopping grpc server...")
		a.grpcServer.GracefulStop()
	}

	// 3. Close Listener
	if a.grpcListener != nil {
		_ = a.grpcListener.Close()
	}

	// 4. Close gRPC Client connections
	if a.parkingConn != nil {
		_ = a.parkingConn.Close()
	}
	if a.userConn != nil {
		_ = a.userConn.Close()
	}

	// 5. Close NATS
	if a.nats != nil {
		a.nats.Close()
	}

	// 6. Close Databases
	if a.cache != nil {
		_ = a.cache.Close()
	}
	if a.db != nil {
		a.db.Close()
	}

	log.Println("app stopped gracefully")
	return nil
}
