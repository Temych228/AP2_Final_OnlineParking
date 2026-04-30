package app

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/config"
)

type App struct {
	cfg    *config.Config
	server *http.Server
	db     *pgxpool.Pool
}

func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	db, err := pgxpool.New(ctx, cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		return nil, err
	}

	log.Println("PostgreSQL connected")

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	server := &http.Server{
		Addr:    cfg.Address(),
		Handler: router,
	}

	return &App{
		cfg:    cfg,
		server: server,
		db:     db,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	log.Printf("server started on %s", a.cfg.Address())
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	a.db.Close()
	return a.server.Shutdown(ctx)
}
