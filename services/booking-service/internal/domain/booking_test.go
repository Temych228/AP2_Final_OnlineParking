package domain_test

import (
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/domain"
)

func TestBookingCanTransitionTo(t *testing.T) {
	cases := []struct {
		from domain.BookingStatus
		to   domain.BookingStatus
		ok   bool
	}{
		{domain.StatusPending, domain.StatusConfirmed, true},
		{domain.StatusPending, domain.StatusCancelled, true},
		{domain.StatusPending, domain.StatusActive, false},
		{domain.StatusConfirmed, domain.StatusActive, true},
		{domain.StatusConfirmed, domain.StatusCancelled, true},
		{domain.StatusActive, domain.StatusCompleted, true},
		{domain.StatusActive, domain.StatusCancelled, false},
		{domain.StatusCompleted, domain.StatusCancelled, false},
	}
	for _, tt := range cases {
		b := &domain.Booking{Status: tt.from}
		got := b.CanTransitionTo(tt.to)
		if got != tt.ok {
			t.Errorf("CanTransitionTo(%s→%s) = %v, want %v", tt.from, tt.to, got, tt.ok)
		}
	}
}

func TestCreateInputValidate(t *testing.T) {
	now := time.Now()
	valid := domain.CreateInput{
		UserID:       "user-1",
		ParkingID:    1,
		SpotID:       1,
		VehiclePlate: "A001AA",
		StartTime:    now,
		EndTime:      now.Add(time.Hour),
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}

	invalid := valid
	invalid.EndTime = now.Add(-time.Hour)
	if err := invalid.Validate(); err == nil {
		t.Error("expected error for invalid time range")
	}
}
