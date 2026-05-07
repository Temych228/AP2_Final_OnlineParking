package domain

import (
	"errors"
	"strings"
	"time"
)

type BookingStatus string

const (
	StatusPending   BookingStatus = "pending"
	StatusConfirmed BookingStatus = "confirmed"
	StatusActive    BookingStatus = "active"
	StatusCompleted BookingStatus = "completed"
	StatusCancelled BookingStatus = "cancelled"
)

type Booking struct {
	ID           string        `json:"id"`
	UserID       string        `json:"user_id"`
	ParkingID    int64         `json:"parking_id"`
	SpotID       int64         `json:"spot_id"`
	VehiclePlate string        `json:"vehicle_plate"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Status       BookingStatus `json:"status"`
	CancelReason string        `json:"cancel_reason"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	CancelledAt  *time.Time    `json:"cancelled_at,omitempty"`
}

type CreateInput struct {
	UserID       string
	ParkingID    int64
	SpotID       int64
	VehiclePlate string
	StartTime    time.Time
	EndTime      time.Time
}

type ListFilter struct {
	Page      int
	PageSize  int
	UserID    string
	ParkingID int64
	SpotID    int64
	Status    BookingStatus
}

var (
	ErrBookingNotFound         = errors.New("booking not found")
	ErrInvalidInput            = errors.New("invalid input")
	ErrBookingConflict         = errors.New("booking time conflicts with an existing booking")
	ErrInvalidStatusTransition = errors.New("invalid booking status transition")
)

func (s BookingStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusConfirmed, StatusActive, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

func (b *Booking) CanTransitionTo(next BookingStatus) bool {
	switch b.Status {
	case StatusPending:
		return next == StatusConfirmed || next == StatusCancelled
	case StatusConfirmed:
		return next == StatusActive || next == StatusCancelled
	case StatusActive:
		return next == StatusCompleted
	default:
		return false
	}
}

func (i *CreateInput) Validate() error {
	i.UserID = strings.TrimSpace(i.UserID)
	i.VehiclePlate = normalizeVehiclePlate(i.VehiclePlate)

	if i.UserID == "" || i.ParkingID <= 0 || i.SpotID <= 0 || i.VehiclePlate == "" {
		return ErrInvalidInput
	}

	if i.StartTime.IsZero() || i.EndTime.IsZero() || !i.EndTime.After(i.StartTime) {
		return ErrInvalidInput
	}

	return nil
}

func (f *ListFilter) Normalize() error {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}

	f.UserID = strings.TrimSpace(f.UserID)
	if f.Status != "" && !f.Status.IsValid() {
		return ErrInvalidInput
	}

	return nil
}

func normalizeVehiclePlate(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}
