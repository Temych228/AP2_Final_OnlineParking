package app

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

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

	return &App{
		db: db,
	}, nil
}

func (a *App) Run() {
	parkingRepo := repository.NewParkingRepository(a.db)
	spotRepo := repository.NewSpotRepository(a.db)
	tariffRepo := repository.NewTariffRepository(a.db)

	parkingUsecase := usecase.NewParkingUsecase(parkingRepo)
	spotUsecase := usecase.NewSpotUsecase(spotRepo)
	tariffUsecase := usecase.NewTariffUsecase(tariffRepo)

	fmt.Println("Parking Service started")
	fmt.Println("Database connected successfully")

	_ = parkingUsecase
	_ = spotUsecase
	_ = tariffUsecase
}
