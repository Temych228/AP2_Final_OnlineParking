package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/service"
)

type fakePaymentRepo struct {
	payments map[string]*domain.Payment
	nextID   int
}

func newFakePaymentRepo() *fakePaymentRepo {
	return &fakePaymentRepo{payments: make(map[string]*domain.Payment), nextID: 1}
}

func (r *fakePaymentRepo) Create(_ context.Context, p *domain.Payment) error {
	if p.ID == "" {
		p.ID = "pay-fake-id"
	}
	cp := *p
	r.payments[p.ID] = &cp
	return nil
}

func (r *fakePaymentRepo) GetByID(_ context.Context, id string) (*domain.Payment, error) {
	if p, ok := r.payments[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, errors.New("payment not found")
}

func (r *fakePaymentRepo) GetByBookingID(_ context.Context, bookingID string) (*domain.Payment, error) {
	for _, p := range r.payments {
		if p.BookingID == bookingID {
			cp := *p
			return &cp, nil
		}
	}
	return nil, errors.New("payment not found")
}

func (r *fakePaymentRepo) List(_ context.Context, filter domain.ListPaymentsFilter) ([]domain.Payment, error) {
	var result []domain.Payment
	for _, p := range r.payments {
		if filter.Status != "" && string(p.Status) != filter.Status {
			continue
		}
		if filter.UserID != "" && p.UserID != filter.UserID {
			continue
		}
		result = append(result, *p)
	}
	return result, nil
}

func (r *fakePaymentRepo) MarkPaid(_ context.Context, id, providerID string) (*domain.Payment, error) {
	p, ok := r.payments[id]
	if !ok {
		return nil, errors.New("payment not found")
	}
	now := time.Now().UTC()
	p.Status = domain.StatusPaid
	p.ProviderPaymentID = providerID
	p.PaidAt = &now
	p.UpdatedAt = now
	cp := *p
	return &cp, nil
}

func (r *fakePaymentRepo) Cancel(_ context.Context, id string) (*domain.Payment, error) {
	p, ok := r.payments[id]
	if !ok {
		return nil, errors.New("payment not found")
	}
	p.Status = domain.StatusCancelled
	p.UpdatedAt = time.Now().UTC()
	cp := *p
	return &cp, nil
}

func (r *fakePaymentRepo) HasPaidPaymentForBooking(_ context.Context, bookingID string) (bool, error) {
	for _, p := range r.payments {
		if p.BookingID == bookingID && p.Status == domain.StatusPaid {
			return true, nil
		}
	}
	return false, nil
}

func newPaymentServiceFake() (*service.PaymentService, *fakePaymentRepo) {
	repo := newFakePaymentRepo()
	svc := service.NewPaymentServiceWithRepo(repo, nil, nil, nil, nil)
	return svc, repo
}

func seedPayment(repo *fakePaymentRepo, id, bookingID, userID string, status domain.PaymentStatus) {
	now := time.Now().UTC()
	repo.payments[id] = &domain.Payment{
		ID:        id,
		BookingID: bookingID,
		UserID:    userID,
		ParkingID: 1,
		SpotID:    10,
		Amount:    1500,
		Method:    domain.MethodCard,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func newPaymentServiceWithoutDependencies() *service.PaymentService {
	return service.NewPaymentService(nil, nil, nil, nil, nil)
}

func TestPaymentServiceCreatePaymentValidationErrors(t *testing.T) {
	svc := newPaymentServiceWithoutDependencies()

	tests := []struct {
		name      string
		input     domain.CreatePaymentInput
		wantError string
	}{
		{
			name:      "missing booking id",
			input:     domain.CreatePaymentInput{BookingID: "", Method: domain.MethodCard},
			wantError: "invalid input",
		},
		{
			name:      "missing payment method",
			input:     domain.CreatePaymentInput{BookingID: "11111111-1111-1111-1111-111111111111", Method: ""},
			wantError: "invalid input",
		},
		{
			name:      "invalid payment method",
			input:     domain.CreatePaymentInput{BookingID: "11111111-1111-1111-1111-111111111111", Method: "invalid"},
			wantError: "invalid payment method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreatePayment(context.Background(), tt.input)
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
	svc := newPaymentServiceWithoutDependencies()

	tests := []struct {
		name      string
		action    func() error
		wantError string
	}{
		{
			name: "get payment requires id",
			action: func() error {
				_, err := svc.GetPayment(context.Background(), "")
				return err
			},
			wantError: "payment id is required",
		},
		{
			name: "get payment by booking requires booking id",
			action: func() error {
				_, err := svc.GetPaymentByBooking(context.Background(), "")
				return err
			},
			wantError: "booking id is required",
		},
		{
			name: "cancel payment requires id",
			action: func() error {
				_, err := svc.CancelPayment(context.Background(), "")
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

func TestPaymentService_GetPayment(t *testing.T) {
	svc, repo := newPaymentServiceFake()
	ctx := context.Background()

	seedPayment(repo, "pay-1", "booking-1", "user-1", domain.StatusPaid)

	got, err := svc.GetPayment(ctx, "pay-1")
	if err != nil {
		t.Fatalf("GetPayment: %v", err)
	}
	if got.ID != "pay-1" || got.BookingID != "booking-1" {
		t.Fatalf("unexpected payment: %+v", got)
	}
	if got.Status != domain.StatusPaid {
		t.Fatalf("expected paid status, got %s", got.Status)
	}

	if _, err := svc.GetPayment(ctx, "missing-id"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestPaymentService_GetPaymentByBooking(t *testing.T) {
	svc, repo := newPaymentServiceFake()
	ctx := context.Background()

	seedPayment(repo, "pay-2", "booking-2", "user-2", domain.StatusPaid)

	got, err := svc.GetPaymentByBooking(ctx, "booking-2")
	if err != nil {
		t.Fatalf("GetPaymentByBooking: %v", err)
	}
	if got.BookingID != "booking-2" {
		t.Fatalf("unexpected booking id: %s", got.BookingID)
	}

	if _, err := svc.GetPaymentByBooking(ctx, "missing-booking"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestPaymentService_ListPayments(t *testing.T) {
	svc, repo := newPaymentServiceFake()
	ctx := context.Background()

	seedPayment(repo, "pay-10", "b-10", "user-A", domain.StatusPaid)
	seedPayment(repo, "pay-11", "b-11", "user-A", domain.StatusPending)
	seedPayment(repo, "pay-12", "b-12", "user-B", domain.StatusCancelled)

	all, err := svc.ListPayments(ctx, domain.ListPaymentsFilter{})
	if err != nil {
		t.Fatalf("ListPayments all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 payments, got %d", len(all))
	}

	paid, err := svc.ListPayments(ctx, domain.ListPaymentsFilter{Status: string(domain.StatusPaid)})
	if err != nil {
		t.Fatalf("ListPayments paid: %v", err)
	}
	if len(paid) != 1 || paid[0].ID != "pay-10" {
		t.Fatalf("expected 1 paid payment, got %d", len(paid))
	}

	byUser, err := svc.ListPayments(ctx, domain.ListPaymentsFilter{UserID: "user-A"})
	if err != nil {
		t.Fatalf("ListPayments userA: %v", err)
	}
	if len(byUser) != 2 {
		t.Fatalf("expected 2 payments for user-A, got %d", len(byUser))
	}
}

func TestPaymentService_CancelPayment(t *testing.T) {
	svc, repo := newPaymentServiceFake()
	ctx := context.Background()

	seedPayment(repo, "pay-20", "b-20", "user-3", domain.StatusPending)
	cancelled, err := svc.CancelPayment(ctx, "pay-20")
	if err != nil {
		t.Fatalf("CancelPayment: %v", err)
	}
	if cancelled.Status != domain.StatusCancelled {
		t.Fatalf("expected cancelled, got %s", cancelled.Status)
	}

	// Paid → cancel должен вернуть ошибку
	seedPayment(repo, "pay-21", "b-21", "user-3", domain.StatusPaid)
	if _, err := svc.CancelPayment(ctx, "pay-21"); err == nil {
		t.Fatal("expected error cancelling paid payment")
	} else if !strings.Contains(err.Error(), "only pending payment can be cancelled") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := svc.CancelPayment(ctx, "missing"); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestPaymentService_CancelPayment_AlreadyCancelled(t *testing.T) {
	svc, repo := newPaymentServiceFake()
	ctx := context.Background()

	seedPayment(repo, "pay-30", "b-30", "user-4", domain.StatusCancelled)
	_, err := svc.CancelPayment(ctx, "pay-30")
	if err == nil || !strings.Contains(err.Error(), "only pending payment can be cancelled") {
		t.Fatalf("expected 'only pending' error, got: %v", err)
	}
}
