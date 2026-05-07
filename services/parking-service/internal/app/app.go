package app

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	httpHandler "parking-service/internal/delivery/http"
	"parking-service/internal/repository"
	"parking-service/internal/usecase"
)

type App struct {
	db *sql.DB
}

func NewApp() (*App, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found, using system env")
	}

	dbURL := os.Getenv("DB_PARKING")
	if dbURL == "" {
		return nil, fmt.Errorf("DB_PARKING is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &App{db: db}, nil
}

func (a *App) Run() {
	parkingRepo := repository.NewParkingRepository(a.db)
	spotRepo := repository.NewSpotRepository(a.db)
	tariffRepo := repository.NewTariffRepository(a.db)

	parkingUsecase := usecase.NewParkingUsecase(parkingRepo)
	spotUsecase := usecase.NewSpotUsecase(spotRepo)
	tariffUsecase := usecase.NewTariffUsecase(tariffRepo)

	parkingHandler := httpHandler.NewParkingHandler(parkingUsecase)
	spotHandler := httpHandler.NewSpotHandler(spotUsecase)
	tariffHandler := httpHandler.NewTariffHandler(tariffUsecase)

	router := gin.Default()

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

	fmt.Println("Parking Service started on port 8080")
	fmt.Println("Database connected successfully")

	err := router.Run(":8080")
	if err != nil {
		log.Fatal(err)
	}
}
