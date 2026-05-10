package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"payment-service/internal/domain"
	"payment-service/internal/integration"
	"payment-service/internal/publisher"
	"payment-service/internal/repository"
)

type PaymentService struct {
	repo               *repository.PaymentRepository
	bookingIntegration *integration.BookingIntegration
	parkingIntegration *integration.ParkingIntegration
	publisher          *publisher.NATSPublisher
	cache              *redis.Client
}

func NewPaymentService(
	repo *repository.PaymentRepository,
	bookingIntegration *integration.BookingIntegration,
	parkingIntegration *integration.ParkingIntegration,
	publisher *publisher.NATSPublisher,
) *PaymentService {
	return &PaymentService{
		repo:               repo,
		bookingIntegration: bookingIntegration,
		parkingIntegration: parkingIntegration,
		publisher:          publisher,
	}
}

func (s *PaymentService) SetCache(cache *redis.Client) {
	s.cache = cache
}

func (s *PaymentService) CreatePayment(ctx context.Context, input domain.CreatePaymentInput) (*domain.Payment, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	if s.cache != nil {
		lockKey := "payment:lock:" + input.BookingID
		ok, err := s.cache.SetNX(ctx, lockKey, "processing", 30*time.Second).Result()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("payment already in progress for this booking")
		}
		defer func() {
			_, _ = s.cache.Del(context.Background(), lockKey).Result()
		}()
	}

	booking, err := s.bookingIntegration.GetBooking(ctx, input.BookingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking: %w", err)
	}

	if booking.Status == "cancelled" || booking.Status == "completed" {
		return nil, errors.New("booking cannot be paid")
	}

	hasPaidPayment, err := s.repo.HasPaidPaymentForBooking(ctx, input.BookingID)
	if err != nil {
		return nil, err
	}
	if hasPaidPayment {
		return nil, errors.New("booking is already paid")
	}

	hours := booking.EndTime.Sub(booking.StartTime).Hours()
	if hours <= 0 {
		return nil, errors.New("invalid booking time range")
	}

	hours = math.Ceil(hours)

	amount, err := s.parkingIntegration.CalculatePrice(ctx, booking.ParkingID, hours)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate payment amount: %w", err)
	}

	now := time.Now().UTC()

	payment := &domain.Payment{
		ID:            uuid.NewString(),
		BookingID:     booking.ID,
		UserID:        booking.UserID,
		ParkingID:     booking.ParkingID,
		SpotID:        booking.SpotID,
		Amount:        amount,
		Method:        input.Method,
		Status:        domain.StatusPending,
		FailureReason: "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.Create(ctx, payment); err != nil {
		return nil, err
	}

	providerPaymentID := "local-" + uuid.NewString()

	paidPayment, err := s.repo.MarkPaid(ctx, payment.ID, providerPaymentID)
	if err != nil {
		return nil, err
	}

	if err := s.bookingIntegration.ConfirmBooking(ctx, booking.ID); err != nil {
		return nil, fmt.Errorf("payment created, but booking confirmation failed: %w", err)
	}

	if s.publisher != nil {
		if err := s.publisher.PublishPaymentSuccess(ctx, paidPayment.UserID, paidPayment.BookingID, paidPayment.Amount); err != nil {
			return nil, fmt.Errorf("payment created, but notification event failed: %w", err)
		}
	}

	return paidPayment, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id string) (*domain.Payment, error) {
	if id == "" {
		return nil, errors.New("payment id is required")
	}

	return s.repo.GetByID(ctx, id)
}

func (s *PaymentService) GetPaymentByBooking(ctx context.Context, bookingID string) (*domain.Payment, error) {
	if bookingID == "" {
		return nil, errors.New("booking id is required")
	}

	return s.repo.GetByBookingID(ctx, bookingID)
}

func (s *PaymentService) ListPayments(ctx context.Context, filter domain.ListPaymentsFilter) ([]domain.Payment, error) {
	return s.repo.List(ctx, filter)
}

func (s *PaymentService) CancelPayment(ctx context.Context, id string) (*domain.Payment, error) {
	if id == "" {
		return nil, errors.New("payment id is required")
	}

	payment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if payment.Status != domain.StatusPending {
		return nil, errors.New("only pending payment can be cancelled")
	}

	return s.repo.Cancel(ctx, id)
}
