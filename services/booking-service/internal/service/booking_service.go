package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/publisher"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Repository interface {
	Create(ctx context.Context, input domain.CreateInput) (*domain.Booking, error)
	GetByID(ctx context.Context, id string) (*domain.Booking, error)
	List(ctx context.Context, filter domain.ListFilter) ([]*domain.Booking, int, error)
	UpdateStatus(ctx context.Context, id string, status domain.BookingStatus, cancelReason string) (*domain.Booking, error)
}

type UserLookup interface {
	GetUserEmail(ctx context.Context, userID string) (string, error)
}

type ParkingLookup interface {
	GetParkingLot(ctx context.Context, parkingLotID string) error
	GetParkingSpotParkingLotID(ctx context.Context, parkingSpotID string) (string, error)
}

type BookingService struct {
	repo      Repository
	users     UserLookup
	parking   ParkingLookup
	publisher *publisher.NATSPublisher
	now       func() time.Time
}

func New(repo Repository, users UserLookup, parking ParkingLookup, pub *publisher.NATSPublisher) *BookingService {
	return &BookingService{
		repo:      repo,
		users:     users,
		parking:   parking,
		publisher: pub,
		now:       time.Now,
	}
}

func (s *BookingService) CreateBooking(ctx context.Context, input domain.CreateInput) (*domain.Booking, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	if s.users != nil {
		if _, err := s.users.GetUserEmail(ctx, input.UserID); err != nil {
			return nil, normalizeDependencyError(err, domain.ErrUserNotFound)
		}
	}

	parkingID := strconv.FormatInt(input.ParkingID, 10)
	spotID := strconv.FormatInt(input.SpotID, 10)

	if s.parking != nil {
		if err := s.parking.GetParkingLot(ctx, parkingID); err != nil {
			return nil, normalizeDependencyError(err, domain.ErrParkingLotNotFound)
		}

		spotParkingLotID, err := s.parking.GetParkingSpotParkingLotID(ctx, spotID)
		if err != nil {
			return nil, normalizeDependencyError(err, domain.ErrParkingSpotNotFound)
		}

		if spotParkingLotID != parkingID {
			return nil, domain.ErrInvalidInput
		}
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
	booking, err := s.transition(ctx, id, domain.StatusConfirmed, "")
	if err != nil {
		return nil, err
	}

	if s.publisher != nil {
		userEmail := ""
		if s.users != nil {
			email, err := s.users.GetUserEmail(ctx, booking.UserID)
			if err == nil {
				userEmail = email
			}
		}
		if err := s.publisher.PublishBookingConfirmed(booking, userEmail); err != nil {
			return nil, err
		}
	}

	return booking, nil
}

func (s *BookingService) CancelBooking(ctx context.Context, id, reason string) (*domain.Booking, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "cancelled"
	}

	booking, err := s.transition(ctx, id, domain.StatusCancelled, reason)
	if err != nil {
		return nil, err
	}

	if s.publisher != nil {
		userEmail := ""
		if s.users != nil {
			email, err := s.users.GetUserEmail(ctx, booking.UserID)
			if err == nil {
				userEmail = email
			}
		}
		if err := s.publisher.PublishBookingCancelled(booking, userEmail, reason); err != nil {
			return nil, err
		}
	}

	return booking, nil
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

func normalizeDependencyError(err error, notFound error) error {
	if err == nil {
		return nil
	}

	if status.Code(err) == codes.NotFound {
		return notFound
	}

	if status.Code(err) == codes.InvalidArgument {
		return domain.ErrInvalidInput
	}

	return fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
}
