package domain

import (
	"errors"
	"strings"
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
	ID                string        `json:"id"`
	BookingID         string        `json:"booking_id"`
	UserID            string        `json:"user_id"`
	UserEmail         string        `json:"user_email,omitempty"`
	ParkingID         int64         `json:"parking_id"`
	SpotID            int64         `json:"spot_id"`
	Amount            float64       `json:"amount"`
	Method            PaymentMethod `json:"method"`
	Status            PaymentStatus `json:"status"`
	ProviderPaymentID string        `json:"provider_payment_id,omitempty"`
	FailureReason     string        `json:"failure_reason,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	PaidAt            *time.Time    `json:"paid_at,omitempty"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type CreatePaymentInput struct {
	BookingID string        `json:"booking_id"`
	Method    PaymentMethod `json:"method"`
}

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidMethod      = errors.New("invalid payment method")
	ErrBookingNotFound    = errors.New("booking not found")
	ErrBookingAlreadyPaid = errors.New("booking is already paid")
	ErrPaymentAlreadyPaid = errors.New("payment is already paid")
)

func (i *CreatePaymentInput) Validate() error {
	i.BookingID = strings.TrimSpace(i.BookingID)
	i.Method = PaymentMethod(strings.TrimSpace(string(i.Method)))

	if i.BookingID == "" {
		return ErrInvalidInput
	}

	if i.Method == "" {
		return ErrInvalidInput
	}

	switch i.Method {
	case MethodCard, MethodCash, MethodKaspi:
		return nil
	default:
		return ErrInvalidMethod
	}
}

type ListPaymentsFilter struct {
	UserID string
	Status string
}
