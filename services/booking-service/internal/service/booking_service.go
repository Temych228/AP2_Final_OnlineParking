package service

import (
	"context"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/domain"
)

type Repository interface {
	Create(ctx context.Context, input domain.CreateInput) (*domain.Booking, error)
	GetByID(ctx context.Context, id string) (*domain.Booking, error)
	List(ctx context.Context, filter domain.ListFilter) ([]*domain.Booking, int, error)
	UpdateStatus(ctx context.Context, id string, status domain.BookingStatus, cancelReason string) (*domain.Booking, error)
}

type BookingService struct {
	repo Repository
	now  func() time.Time
}

func New(repo Repository) *BookingService {
	return &BookingService{
		repo: repo,
		now:  time.Now,
	}
}

func (s *BookingService) CreateBooking(ctx context.Context, input domain.CreateInput) (*domain.Booking, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, input)
}

func (s *BookingService) GetBooking(ctx context.Context, id string) (*domain.Booking, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *BookingService) ListBookings(ctx context.Context, filter domain.ListFilter) ([]*domain.Booking, int, error) {
	if err := filter.Normalize(); err != nil {
		return nil, 0, err
	}

	return s.repo.List(ctx, filter)
}

func (s *BookingService) ConfirmBooking(ctx context.Context, id string) (*domain.Booking, error) {
	return s.transition(ctx, id, domain.StatusConfirmed, "")
}

func (s *BookingService) CancelBooking(ctx context.Context, id, reason string) (*domain.Booking, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "cancelled"
	}

	return s.transition(ctx, id, domain.StatusCancelled, reason)
}

func (s *BookingService) StartBooking(ctx context.Context, id string) (*domain.Booking, error) {
	booking, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if booking.Status != domain.StatusConfirmed {
		return nil, domain.ErrInvalidStatusTransition
	}

	if s.now().Before(booking.StartTime) {
		return nil, domain.ErrInvalidStatusTransition
	}

	return s.repo.UpdateStatus(ctx, id, domain.StatusActive, "")
}

func (s *BookingService) CompleteBooking(ctx context.Context, id string) (*domain.Booking, error) {
	return s.transition(ctx, id, domain.StatusCompleted, "")
}

func (s *BookingService) transition(ctx context.Context, id string, next domain.BookingStatus, cancelReason string) (*domain.Booking, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.ErrInvalidInput
	}

	booking, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !booking.CanTransitionTo(next) {
		return nil, domain.ErrInvalidStatusTransition
	}

	return s.repo.UpdateStatus(ctx, id, next, cancelReason)
}
