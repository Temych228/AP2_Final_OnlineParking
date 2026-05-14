package integration_test

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	httpHandler "github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/delivery/http"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/usecase"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DB_PARKING")
	}
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL or DB_PARKING to run integration tests")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("failed to ping test database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS parkings (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			address TEXT NOT NULL,
			total_spots INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS spots (
			id SERIAL PRIMARY KEY,
			parking_id INT NOT NULL REFERENCES parkings(id) ON DELETE CASCADE,
			number VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'AVAILABLE',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS tariffs (
			id SERIAL PRIMARY KEY,
			parking_id INT NOT NULL REFERENCES parkings(id) ON DELETE CASCADE,
			price_per_hour DECIMAL(10,2) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		TRUNCATE TABLE tariffs, spots, parkings RESTART IDENTITY CASCADE;
	`)
	if err != nil {
		_ = db.Close()
		t.Fatalf("failed to prepare test tables: %v", err)
	}

	return db
}

func setupRouter(db *sql.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)

	parkingRepo := repository.NewParkingRepository(db, nil, time.Minute)
	parkingUC := usecase.NewParkingUsecase(parkingRepo)
	parkingHandler := httpHandler.NewParkingHandler(parkingUC)

	router := gin.New()
	router.POST("/parkings", parkingHandler.CreateParking)
	router.GET("/parkings/:id", parkingHandler.GetParking)

	return router
}

func TestParkingHTTPCreateAndGetIntegration(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	router := setupRouter(db)

	createBody := bytes.NewBufferString(`{"name":"Mega Parking","address":"Astana","total_spots":50}`)
	createReq := httptest.NewRequest(http.MethodPost, "/parkings", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()

	router.ServeHTTP(createResp, createReq)

	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body: %s", createResp.Code, createResp.Body.String())
	}
	if !strings.Contains(createResp.Body.String(), "Mega Parking") {
		t.Fatalf("expected created parking in response, got %s", createResp.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/parkings/1", nil)
	getResp := httptest.NewRecorder()

	router.ServeHTTP(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", getResp.Code, getResp.Body.String())
	}
	if !strings.Contains(getResp.Body.String(), "Mega Parking") {
		t.Fatalf("expected fetched parking in response, got %s", getResp.Body.String())
	}
}
