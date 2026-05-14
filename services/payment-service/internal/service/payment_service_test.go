package service_test

import (
	"context"
	"strings"
	"testing"

	"payment-service/internal/domain"
	"payment-service/internal/service"
)

func newPaymentServiceWithoutDependencies() *service.PaymentService {
	return service.NewPaymentService(nil, nil, nil, nil)
}

func TestPaymentServiceCreatePaymentValidationErrors(t *testing.T) {
	paymentService := newPaymentServiceWithoutDependencies()

	tests := []struct {
		name      string
		input     domain.CreatePaymentInput
		wantError string
	}{
		{
			name: "missing booking id",
			input: domain.CreatePaymentInput{
				BookingID: "",
				Method:    domain.MethodCard,
			},
			wantError: "booking_id is required",
		},
		{
			name: "missing payment method",
			input: domain.CreatePaymentInput{
				BookingID: "11111111-1111-1111-1111-111111111111",
				Method:    "",
			},
			wantError: "method is required",
		},
		{
			name: "invalid payment method",
			input: domain.CreatePaymentInput{
				BookingID: "11111111-1111-1111-1111-111111111111",
				Method:    "invalid",
			},
			wantError: "invalid payment method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := paymentService.CreatePayment(context.Background(), tt.input)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}

func TestPaymentServiceRequiredIDs(t *testing.T) {
	paymentService := newPaymentServiceWithoutDependencies()

	tests := []struct {
		name      string
		action    func() error
		wantError string
	}{
		{
			name: "get payment requires id",
			action: func() error {
				_, err := paymentService.GetPayment(context.Background(), "")
				return err
			},
			wantError: "payment id is required",
		},
		{
			name: "get payment by booking requires booking id",
			action: func() error {
				_, err := paymentService.GetPaymentByBooking(context.Background(), "")
				return err
			},
			wantError: "booking id is required",
		},
		{
			name: "cancel payment requires id",
			action: func() error {
				_, err := paymentService.CancelPayment(context.Background(), "")
				return err
			},
			wantError: "payment id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action()
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}
