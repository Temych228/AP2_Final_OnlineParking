package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
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

type eventJob struct {
	subject string
	data    []byte
}

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

	jobs        chan eventJob
	workersDone sync.WaitGroup
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

	numWorkers := cfg.NumWorkers
	if numWorkers <= 0 {
		numWorkers = 5
	}

	jobs := make(chan eventJob, numWorkers*64)

	app := &App{
		cfg:           cfg,
		db:            db,
		cache:         cache,
		nats:          nc,
		grpcServer:    grpcSrv,
		httpServer:    httpServer,
		metricsServer: metricsServer,
		svc:           svc,
		jobs:          jobs,
	}

	app.startWorkers(numWorkers)

	if err := app.subscribeNATS(); err != nil {
		close(jobs)
		nc.Close()
		db.Close()
		_ = cache.Close()
		return nil, err
	}

	return app, nil
}

func (a *App) startWorkers(n int) {
	log.Printf("[nats-workers] starting %d workers", n)
	for i := 0; i < n; i++ {
		a.workersDone.Add(1)
		go a.runWorker(i)
	}
}

func (a *App) runWorker(id int) {
	defer a.workersDone.Done()
	log.Printf("[worker-%d] started, listening for events", id)

	for job := range a.jobs {
		start := time.Now()
		log.Printf("[worker-%d] START subject=%s payload_bytes=%d", id, job.subject, len(job.data))

		err := a.svc.HandleEvent(context.Background(), job.subject, job.data)
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("[worker-%d] ERROR subject=%s duration=%s err=%v", id, job.subject, elapsed, err)
		} else {
			log.Printf("[worker-%d] OK    subject=%s duration=%s", id, job.subject, elapsed)
		}
	}

	log.Printf("[worker-%d] stopped (channel closed)", id)
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
		sub := subject // захват переменной для замыкания
		_, err := a.nats.QueueSubscribe(sub, "notification-workers", func(msg *nats.Msg) {
			job := eventJob{subject: msg.Subject, data: msg.Data}
			select {
			case a.jobs <- job:
				log.Printf("[nats] enqueued subject=%s queue_len=%d", msg.Subject, len(a.jobs))
			default:
				log.Printf("[nats] WARNING: job queue full (%d/%d), dropping subject=%s",
					len(a.jobs), cap(a.jobs), msg.Subject)
			}
		})
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", sub, err)
		}
		log.Printf("[nats] subscribed subject=%s queue=notification-workers", sub)
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

	log.Printf("notification-service started http=%s grpc=%s metrics=%s workers=%d",
		a.cfg.HTTPAddress(), a.cfg.GRPCAddress(), a.cfg.MetricsAddress(), a.cfg.NumWorkers)
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if a.nats != nil {
		log.Printf("[shutdown] draining NATS subscriptions...")
		_ = a.nats.Drain()
	}

	if a.jobs != nil {
		close(a.jobs)
		log.Printf("[shutdown] waiting for workers to finish...")
		a.workersDone.Wait()
		log.Printf("[shutdown] all workers done")
	}

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
