package domain

import (
	"errors"
	"time"
)

type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusPaid      PaymentStatus = "paid"
	StatusFailed    PaymentStatus = "failed"
	StatusCancelled PaymentStatus = "cancelled"
)

type PaymentMethod string

const (
	MethodCard  PaymentMethod = "card"
	MethodCash  PaymentMethod = "cash"
	MethodKaspi PaymentMethod = "kaspi"
)

type Payment struct {
	ID                string
	BookingID         string
	UserID            string
	ParkingID         int64
	SpotID            int64
	Amount            float64
	Method            PaymentMethod
	Status            PaymentStatus
	ProviderPaymentID string
	FailureReason     string
	CreatedAt         time.Time
	PaidAt            *time.Time
	UpdatedAt         time.Time
}

type CreatePaymentInput struct {
	BookingID string        `json:"booking_id"`
	Method    PaymentMethod `json:"method"`
}

func (i CreatePaymentInput) Validate() error {
	if i.BookingID == "" {
		return errors.New("booking_id is required")
	}

	if i.Method == "" {
		return errors.New("method is required")
	}

	switch i.Method {
	case MethodCard, MethodCash, MethodKaspi:
		return nil
	default:
		return errors.New("invalid payment method")
	}
}

type ListPaymentsFilter struct {
	UserID string
	Status string
}
