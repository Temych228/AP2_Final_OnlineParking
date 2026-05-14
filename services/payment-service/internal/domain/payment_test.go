package domain_test

import (
	"strings"
	"testing"

	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/domain"
)

func TestCreatePaymentInputValidationErrors(t *testing.T) {
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
			wantError: "invalid input",
		},
		{
			name: "missing method",
			input: domain.CreatePaymentInput{
				BookingID: "11111111-1111-1111-1111-111111111111",
				Method:    "",
			},
			wantError: "invalid input",
		},
		{
			name: "invalid method",
			input: domain.CreatePaymentInput{
				BookingID: "11111111-1111-1111-1111-111111111111",
				Method:    "bitcoin",
			},
			wantError: "invalid payment method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}

func TestCreatePaymentInputValidMethods(t *testing.T) {
	validMethods := []domain.PaymentMethod{
		domain.MethodCard,
		domain.MethodCash,
		domain.MethodKaspi,
	}

	for _, method := range validMethods {
		t.Run(string(method), func(t *testing.T) {
			input := domain.CreatePaymentInput{
				BookingID: "11111111-1111-1111-1111-111111111111",
				Method:    method,
			}

			if err := input.Validate(); err != nil {
				t.Fatalf("expected valid method %q, got error: %v", method, err)
			}
		})
	}
}
