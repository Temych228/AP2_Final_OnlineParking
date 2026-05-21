package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/integration"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/publisher"
	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/repository"
)

type PaymentRepo interface {
	Create(ctx context.Context, p *domain.Payment) error
	GetByID(ctx context.Context, id string) (*domain.Payment, error)
	GetByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error)
	List(ctx context.Context, filter domain.ListPaymentsFilter) ([]domain.Payment, error)
	MarkPaid(ctx context.Context, id, providerPaymentID string) (*domain.Payment, error)
	Cancel(ctx context.Context, id string) (*domain.Payment, error)
	HasPaidPaymentForBooking(ctx context.Context, bookingID string) (bool, error)
}

type PaymentService struct {
	repo               PaymentRepo // было: *repository.PaymentRepository
	bookingIntegration *integration.BookingIntegration
	parkingIntegration *integration.ParkingIntegration
	userIntegration    *integration.UserIntegration
	publisher          *publisher.NATSPublisher
	cache              *redis.Client
}

func NewPaymentService(
	repo *repository.PaymentRepository,
	bookingIntegration *integration.BookingIntegration,
	parkingIntegration *integration.ParkingIntegration,
	userIntegration *integration.UserIntegration,
	pub *publisher.NATSPublisher,
) *PaymentService {
	return &PaymentService{
		repo:               repo,
		bookingIntegration: bookingIntegration,
		parkingIntegration: parkingIntegration,
		userIntegration:    userIntegration,
		publisher:          pub,
	}
}

func NewPaymentServiceWithRepo(
	repo PaymentRepo,
	bookingIntegration *integration.BookingIntegration,
	parkingIntegration *integration.ParkingIntegration,
	userIntegration *integration.UserIntegration,
	pub *publisher.NATSPublisher,
) *PaymentService {
	return &PaymentService{
		repo:               repo,
		bookingIntegration: bookingIntegration,
		parkingIntegration: parkingIntegration,
		userIntegration:    userIntegration,
		publisher:          pub,
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

	userEmail := booking.UserEmail
	if userEmail == "" && s.userIntegration != nil {
		userEmail, err = s.userIntegration.GetUserEmail(ctx, booking.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user email: %w", err)
		}
	}

	now := time.Now().UTC()
	payment := &domain.Payment{
		ID:        uuid.NewString(),
		BookingID: booking.ID,
		UserID:    booking.UserID,
		UserEmail: userEmail,
		ParkingID: booking.ParkingID,
		SpotID:    booking.SpotID,
		Amount:    amount,
		Method:    input.Method,
		Status:    domain.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
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

	if s.publisher != nil && userEmail != "" {
		if err := s.publisher.PublishPaymentSuccess(ctx, paidPayment.UserID, userEmail, paidPayment.BookingID, paidPayment.Amount); err != nil {
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

func NewPaymentServiceFull(
	repo PaymentRepo,
	bookingIntegration *integration.BookingIntegration,
	parkingIntegration *integration.ParkingIntegration,
	userIntegration *integration.UserIntegration,
	pub *publisher.NATSPublisher,
	cache *redis.Client,
) *PaymentService {
	return &PaymentService{
		repo:               repo,
		bookingIntegration: bookingIntegration,
		parkingIntegration: parkingIntegration,
		userIntegration:    userIntegration,
		publisher:          pub,
		cache:              cache,
	}
}
