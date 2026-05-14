package integration_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	httpdelivery "github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/delivery/http"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/service"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_PAYMENT_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_PAYMENT_DATABASE_URL to run payment integration tests")
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
		CREATE EXTENSION IF NOT EXISTS pgcrypto;

		CREATE TABLE IF NOT EXISTS payments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			booking_id UUID NOT NULL,
			user_id UUID NOT NULL,
			parking_id BIGINT NOT NULL,
			spot_id BIGINT NOT NULL,
			amount NUMERIC(10, 2) NOT NULL,
			method TEXT NOT NULL DEFAULT 'card',
			status TEXT NOT NULL DEFAULT 'pending',
			provider_payment_id TEXT,
			failure_reason TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			paid_at TIMESTAMPTZ NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		TRUNCATE TABLE payments;

		INSERT INTO payments (
			id,
			booking_id,
			user_id,
			parking_id,
			spot_id,
			amount,
			method,
			status,
			provider_payment_id,
			failure_reason,
			created_at,
			paid_at,
			updated_at
		)
		VALUES (
			'11111111-1111-1111-1111-111111111111',
			'22222222-2222-2222-2222-222222222222',
			'33333333-3333-3333-3333-333333333333',
			1,
			10,
			1500.00,
			'card',
			'paid',
			'local-test-provider',
			'',
			NOW(),
			NOW(),
			NOW()
		);
	`)
	if err != nil {
		_ = db.Close()
		t.Fatalf("failed to prepare test database: %v", err)
	}

	return db
}

func setupRouter(db *sql.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)

	paymentRepo := repository.NewPaymentRepository(db)
	paymentService := service.NewPaymentService(paymentRepo, nil, nil, nil, nil)
	paymentHandler := httpdelivery.NewPaymentHandler(paymentService)

	router := gin.New()
	paymentHandler.RegisterRoutes(router)

	return router
}

func TestPaymentHTTPGetAndListIntegration(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	router := setupRouter(db)

	getReq := httptest.NewRequest(
		http.MethodGet,
		"/payments/11111111-1111-1111-1111-111111111111",
		nil,
	)
	getResp := httptest.NewRecorder()

	router.ServeHTTP(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", getResp.Code, getResp.Body.String())
	}
	if !strings.Contains(getResp.Body.String(), "11111111-1111-1111-1111-111111111111") {
		t.Fatalf("expected payment id in response, got %s", getResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/payments?status=paid", nil)
	listResp := httptest.NewRecorder()

	router.ServeHTTP(listResp, listReq)

	if listResp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", listResp.Code, listResp.Body.String())
	}
	if !strings.Contains(listResp.Body.String(), "paid") {
		t.Fatalf("expected paid payment in response, got %s", listResp.Body.String())
	}
}

func TestPaymentHTTPGetByBookingIntegration(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	router := setupRouter(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/payments/booking/22222222-2222-2222-2222-222222222222",
		nil,
	)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "22222222-2222-2222-2222-222222222222") {
		t.Fatalf("expected booking id in response, got %s", resp.Body.String())
	}
}
