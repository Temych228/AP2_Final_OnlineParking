package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/client"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/service"
	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/transport/http"
	"github.com/gin-gonic/gin"
)

type fakeBookingRepo struct {
	bookings map[string]*domain.Booking
	nextID   int
}

func newFakeBookingRepo() *fakeBookingRepo {
	return &fakeBookingRepo{
		bookings: make(map[string]*domain.Booking),
		nextID:   1,
	}
}

func (r *fakeBookingRepo) Create(_ context.Context, input domain.CreateInput) (*domain.Booking, error) {
	id := fmt.Sprintf("booking-%d", r.nextID)
	r.nextID++
	b := &domain.Booking{
		ID:           id,
		UserID:       input.UserID,
		ParkingID:    input.ParkingID,
		SpotID:       input.SpotID,
		VehiclePlate: input.VehiclePlate,
		StartTime:    input.StartTime,
		EndTime:      input.EndTime,
		Status:       domain.StatusPending,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	r.bookings[id] = b
	return b, nil
}

func (r *fakeBookingRepo) GetByID(_ context.Context, id string) (*domain.Booking, error) {
	b, ok := r.bookings[id]
	if !ok {
		return nil, domain.ErrBookingNotFound
	}
	cp := *b
	return &cp, nil
}

func (r *fakeBookingRepo) List(_ context.Context, filter domain.ListFilter) ([]*domain.Booking, int, error) {
	var items []*domain.Booking
	for _, b := range r.bookings {
		if filter.UserID != "" && b.UserID != filter.UserID {
			continue
		}
		if filter.Status != "" && b.Status != filter.Status {
			continue
		}
		cp := *b
		items = append(items, &cp)
	}
	return items, len(items), nil
}

func (r *fakeBookingRepo) UpdateStatus(_ context.Context, id string, status domain.BookingStatus, cancelReason string) (*domain.Booking, error) {
	b, ok := r.bookings[id]
	if !ok {
		return nil, domain.ErrBookingNotFound
	}
	b.Status = status
	b.CancelReason = cancelReason
	b.UpdatedAt = time.Now().UTC()
	cp := *b
	return &cp, nil
}

type fakeUserLookup struct{}

func (f *fakeUserLookup) GetUserEmail(_ context.Context, userID string) (string, error) {
	if userID == "missing-user" {
		return "", errors.New("not found")
	}
	return userID + "@test.com", nil
}

type fakeParkingLookup struct {
	spots []client.SpotInfo
}

func newFakeParkingLookup() *fakeParkingLookup {
	return &fakeParkingLookup{
		spots: []client.SpotInfo{
			{ID: 1, ParkingID: 1, Number: "A1", Status: "AVAILABLE"},
			{ID: 2, ParkingID: 1, Number: "A2", Status: "AVAILABLE"},
		},
	}
}

func (f *fakeParkingLookup) GetParkingLot(_ context.Context, parkingLotID string) error {
	if parkingLotID == "999" {
		return errors.New("not found")
	}
	return nil
}

func (f *fakeParkingLookup) GetParkingSpotParkingLotID(_ context.Context, spotID string) (string, error) {
	if spotID == "1" {
		return "1", nil
	}
	return "", errors.New("not found")
}

func (f *fakeParkingLookup) ListParkingSpots(_ context.Context, parkingLotID string) ([]client.SpotInfo, error) {
	return f.spots, nil
}

func (f *fakeParkingLookup) ReserveSpot(_ context.Context, spotID string) error {
	for i, s := range f.spots {
		if fmt.Sprintf("%d", s.ID) == spotID {
			f.spots[i].Status = "RESERVED"
			return nil
		}
	}
	return nil
}

func (f *fakeParkingLookup) ReleaseSpot(_ context.Context, spotID string) error {
	for i, s := range f.spots {
		if fmt.Sprintf("%d", s.ID) == spotID {
			f.spots[i].Status = "AVAILABLE"
			return nil
		}
	}
	return nil
}

func (f *fakeParkingLookup) UpdateSpotStatus(_ context.Context, spotID string, status string) error {
	return nil
}

func newBookingService() (*service.BookingService, *fakeBookingRepo) {
	repo := newFakeBookingRepo()
	users := &fakeUserLookup{}
	parking := newFakeParkingLookup()
	svc := service.New(repo, users, parking, nil)
	return svc, repo
}

func mustMarshal(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return bytes.NewReader(data)
}

func TestBookingService_Unit(t *testing.T) {
	svc, _ := newBookingService()
	ctx := context.Background()

	now := time.Now().UTC()

	booking, err := svc.CreateBooking(ctx, domain.CreateInput{
		UserID:       "user-1",
		ParkingID:    1,
		VehiclePlate: "A001AA",
		StartTime:    now.Add(time.Hour),
		EndTime:      now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create booking: %v", err)
	}
	if booking.ID == "" || booking.Status != domain.StatusPending {
		t.Fatalf("unexpected booking: %#v", booking)
	}

	got, err := svc.GetBooking(ctx, booking.ID)
	if err != nil {
		t.Fatalf("get booking: %v", err)
	}
	if got.ID != booking.ID {
		t.Fatalf("unexpected get booking: %#v", got)
	}

	bookings, total, err := svc.ListBookings(ctx, domain.ListFilter{
		Page: 1, PageSize: 20, UserID: "user-1",
	})
	if err != nil || total != 1 || len(bookings) != 1 {
		t.Fatalf("unexpected list: total=%d len=%d err=%v", total, len(bookings), err)
	}

	confirmed, err := svc.ConfirmBooking(ctx, booking.ID)
	if err != nil {
		t.Fatalf("confirm booking: %v", err)
	}
	if confirmed.Status != domain.StatusConfirmed {
		t.Fatalf("unexpected status after confirm: %s", confirmed.Status)
	}

	cancelled, err := svc.CancelBooking(ctx, booking.ID, "test cancel")
	if err != nil {
		t.Fatalf("cancel booking: %v", err)
	}
	if cancelled.Status != domain.StatusCancelled {
		t.Fatalf("unexpected status after cancel: %s", cancelled.Status)
	}
}

func TestBookingService_Errors(t *testing.T) {
	svc, _ := newBookingService()
	ctx := context.Background()

	_, err := svc.CreateBooking(ctx, domain.CreateInput{
		UserID:       "",
		ParkingID:    1,
		VehiclePlate: "A001AA",
		StartTime:    time.Now(),
		EndTime:      time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("expected invalid input error")
	}

	_, err = svc.CreateBooking(ctx, domain.CreateInput{
		UserID:       "user-1",
		ParkingID:    999,
		VehiclePlate: "A001AA",
		StartTime:    time.Now().Add(time.Hour),
		EndTime:      time.Now().Add(2 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected parking not found error")
	}

	_, err = svc.GetBooking(ctx, "missing-booking")
	if err == nil {
		t.Fatal("expected booking not found error")
	}

	booking, _ := svc.CreateBooking(ctx, domain.CreateInput{
		UserID:       "user-1",
		ParkingID:    1,
		VehiclePlate: "B002BB",
		StartTime:    time.Now().Add(time.Hour),
		EndTime:      time.Now().Add(2 * time.Hour),
	})
	_, err = svc.CompleteBooking(ctx, booking.ID) // can't complete pending booking
	if err == nil {
		t.Fatal("expected invalid transition error")
	}
}

func TestBookingHTTP_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newBookingService()
	handler := httptransport.New(svc)
	router := gin.New()
	handler.Register(router)

	now := time.Now().UTC()

	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, httptest.NewRequest(http.MethodPost, "/bookings", mustMarshal(t, map[string]any{
		"user_id":       "user-1",
		"parking_id":    1,
		"vehicle_plate": "A001AA",
		"start_time":    now.Add(time.Hour).Format(time.RFC3339),
		"end_time":      now.Add(2 * time.Hour).Format(time.RFC3339),
	})))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResp.Code, createResp.Body.String())
	}

	var created domain.Booking
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected booking id in response")
	}

	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, httptest.NewRequest(http.MethodGet, "/bookings/"+created.ID, nil))
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getResp.Code, getResp.Body.String())
	}

	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, httptest.NewRequest(http.MethodGet, "/bookings?user_id=user-1", nil))
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listResp.Code, listResp.Body.String())
	}

	confirmResp := httptest.NewRecorder()
	router.ServeHTTP(confirmResp, httptest.NewRequest(http.MethodPost, "/bookings/"+created.ID+"/confirm", nil))
	if confirmResp.Code != http.StatusOK {
		t.Fatalf("confirm status = %d body=%s", confirmResp.Code, confirmResp.Body.String())
	}

	cancelResp := httptest.NewRecorder()
	router.ServeHTTP(cancelResp, httptest.NewRequest(http.MethodPost, "/bookings/"+created.ID+"/cancel", mustMarshal(t, map[string]any{
		"reason": "integration test cancel",
	})))
	if cancelResp.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body=%s", cancelResp.Code, cancelResp.Body.String())
	}

	quickResp := httptest.NewRecorder()
	router.ServeHTTP(quickResp, httptest.NewRequest(http.MethodPost, "/bookings/quick", mustMarshal(t, map[string]any{
		"user_id":       "user-2",
		"parking_id":    1,
		"vehicle_plate": "B002BB",
		"hours":         2,
	})))
	if quickResp.Code != http.StatusCreated {
		t.Fatalf("quick booking status = %d body=%s", quickResp.Code, quickResp.Body.String())
	}
}
