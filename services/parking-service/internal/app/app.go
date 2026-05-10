package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	grpcHandler "github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/delivery/grpc"
	httpHandler "github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/delivery/http"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"

	parkingv1 "github.com/Temych228/ap2_protos_go_final/parking/v1"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type App struct {
	db         *sql.DB
	cache      *redis.Client
	httpServer *http.Server
	grpcServer *grpc.Server
	httpLn     net.Listener
	grpcLn     net.Listener
}

func NewApp() (*App, error) {
	_ = godotenv.Load()

	dbURL := strings.TrimSpace(os.Getenv("DB_PARKING"))
	if dbURL == "" {
		dbURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dbURL == "" {
		return nil, fmt.Errorf("DB_PARKING or DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(25)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &App{db: db}, nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.connectRedis(); err != nil {
		return err
	}
	defer a.cache.Close()

	parkingRepo := repository.NewParkingRepository(a.db, a.cache, 5*time.Minute)
	spotRepo := repository.NewSpotRepository(a.db)
	tariffRepo := repository.NewTariffRepository(a.db)

	parkingUC := usecase.NewParkingUsecase(parkingRepo)
	spotUC := usecase.NewSpotUsecase(spotRepo, parkingRepo)
	tariffUC := usecase.NewTariffUsecase(tariffRepo, parkingRepo)

	httpSrv, httpLn, err := buildHTTPServer(parkingUC, spotUC, tariffUC)
	if err != nil {
		return err
	}
	a.httpServer = httpSrv
	a.httpLn = httpLn

	grpcSrv, grpcLn, err := buildGRPCServer(parkingUC, spotUC, tariffUC)
	if err != nil {
		_ = a.httpServer.Close()
		_ = a.httpLn.Close()
		return err
	}
	a.grpcServer = grpcSrv
	a.grpcLn = grpcLn

	errCh := make(chan error, 2)

	go func() {
		log.Printf("HTTP server started on %s", a.httpLn.Addr().String())
		if err := a.httpServer.Serve(a.httpLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		log.Printf("gRPC server started on %s", a.grpcLn.Addr().String())
		if err := a.grpcServer.Serve(a.grpcLn); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var errs []string

	if a.httpServer != nil {
		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if a.grpcServer != nil {
		stopped := make(chan struct{})
		go func() {
			a.grpcServer.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
		case <-shutdownCtx.Done():
			a.grpcServer.Stop()
		}
	}

	if a.httpLn != nil {
		_ = a.httpLn.Close()
	}

	if a.grpcLn != nil {
		_ = a.grpcLn.Close()
	}

	if a.cache != nil {
		_ = a.cache.Close()
	}

	if a.db != nil {
		_ = a.db.Close()
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}

	return nil
}

func RunWithSignal() error {
	app, err := NewApp()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runErr := make(chan error, 1)
	go func() {
		runErr <- app.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		return app.Shutdown(context.Background())
	case err := <-runErr:
		_ = app.Shutdown(context.Background())
		return err
	}
}

func (a *App) connectRedis() error {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		host := strings.TrimSpace(os.Getenv("REDIS_HOST"))
		if host == "" {
			host = "redis"
		}
		port := strings.TrimSpace(os.Getenv("REDIS_PORT"))
		if port == "" {
			port = "6379"
		}
		addr = host + ":" + port
	}

	dbIndex := 4
	if raw := strings.TrimSpace(os.Getenv("REDIS_DB")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			dbIndex = v
		}
	}

	cache := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       dbIndex,
	})

	if err := cache.Ping(context.Background()).Err(); err != nil {
		return err
	}

	a.cache = cache
	log.Println("connected to Redis")

	return nil
}

func buildHTTPServer(
	parkingUC *usecase.ParkingUsecase,
	spotUC *usecase.SpotUsecase,
	tariffUC *usecase.TariffUsecase,
) (*http.Server, net.Listener, error) {
	parkingHandler := httpHandler.NewParkingHandler(parkingUC)
	spotHandler := httpHandler.NewSpotHandler(spotUC)
	tariffHandler := httpHandler.NewTariffHandler(tariffUC)

	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/parkings", parkingHandler.CreateParking)
	router.GET("/parkings/:id", parkingHandler.GetParking)
	router.GET("/parkings", parkingHandler.GetAllParkings)

	router.POST("/spots", spotHandler.CreateSpot)
	router.GET("/spots/:id", spotHandler.GetSpot)
	router.GET("/parkings/:id/spots", spotHandler.GetSpotsByParking)
	router.PATCH("/spots/:id/status", spotHandler.UpdateSpotStatus)
	router.POST("/spots/:id/reserve", spotHandler.ReserveSpot)
	router.POST("/spots/:id/release", spotHandler.ReleaseSpot)
	router.DELETE("/spots/:id", spotHandler.DeleteSpot)

	router.POST("/tariffs", tariffHandler.CreateTariff)
	router.GET("/tariffs/:parking_id", tariffHandler.GetTariff)
	router.PATCH("/tariffs/:parking_id", tariffHandler.UpdateTariff)
	router.GET("/tariffs/:parking_id/calculate", tariffHandler.CalculatePrice)

	port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if port == "" {
		port = "8085"
	}

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, nil, err
	}

	srv := &http.Server{
		Handler: router,
	}

	return srv, ln, nil
}

func buildGRPCServer(
	parkingUC *usecase.ParkingUsecase,
	spotUC *usecase.SpotUsecase,
	tariffUC *usecase.TariffUsecase,
) (*grpc.Server, net.Listener, error) {
	port := strings.TrimSpace(os.Getenv("GRPC_PORT"))
	if port == "" {
		port = "9095"
	}

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, nil, err
	}

	grpcServer := grpc.NewServer()
	grpcGRPCHandler := grpcHandler.NewParkingGRPCHandler(parkingUC, spotUC, tariffUC)

	parkingv1.RegisterParkingServiceServer(grpcServer, grpcGRPCHandler)

	return grpcServer, ln, nil
}
