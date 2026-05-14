package service

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/client"
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
	ListParkingSpots(ctx context.Context, parkingLotID string) ([]client.SpotInfo, error)
	ReserveSpot(ctx context.Context, spotID string) error
	ReleaseSpot(ctx context.Context, spotID string) error
	UpdateSpotStatus(ctx context.Context, spotID string, status string) error
}

type BookingService struct {
	repo      Repository
	users     UserLookup
	parking   ParkingLookup
	publisher *publisher.NATSPublisher
	now       func() time.Time
	rng       *rand.Rand
}

func New(repo Repository, users UserLookup, parking ParkingLookup, pub *publisher.NATSPublisher) *BookingService {
	return &BookingService{
		repo:      repo,
		users:     users,
		parking:   parking,
		publisher: pub,
		now:       time.Now,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
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

	if s.parking != nil {
		if err := s.parking.GetParkingLot(ctx, parkingID); err != nil {
			return nil, normalizeDependencyError(err, domain.ErrParkingLotNotFound)
		}

		if input.SpotID <= 0 {
			spotID, err := s.pickRandomAvailableSpot(ctx, parkingID)
			if err != nil {
				return nil, err
			}
			input.SpotID = spotID
		} else {
			spotParkingLotID, err := s.parking.GetParkingSpotParkingLotID(ctx, strconv.FormatInt(input.SpotID, 10))
			if err != nil {
				return nil, normalizeDependencyError(err, domain.ErrParkingSpotNotFound)
			}
			if spotParkingLotID != parkingID {
				return nil, domain.ErrInvalidInput
			}
		}

		spotID := strconv.FormatInt(input.SpotID, 10)
		if err := s.parking.ReserveSpot(ctx, spotID); err != nil {
			return nil, normalizeParkingError(err)
		}

		booking, err := s.repo.Create(ctx, input)
		if err != nil {
			_ = s.parking.ReleaseSpot(ctx, spotID)
			return nil, err
		}

		return s.enrichBooking(ctx, booking), nil
	}

	booking, err := s.repo.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	return s.enrichBooking(ctx, booking), nil
}

func (s *BookingService) GetBooking(ctx context.Context, id string) (*domain.Booking, error) {
	booking, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.enrichBooking(ctx, booking), nil
}

func (s *BookingService) ListBookings(ctx context.Context, filter domain.ListFilter) ([]*domain.Booking, int, error) {
	if err := filter.Normalize(); err != nil {
		return nil, 0, err
	}

	bookings, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	for i, booking := range bookings {
		bookings[i] = s.enrichBooking(ctx, booking)
	}

	return bookings, total, nil
}

func (s *BookingService) ConfirmBooking(ctx context.Context, id string) (*domain.Booking, error) {
	booking, err := s.transition(ctx, id, domain.StatusConfirmed, "")
	if err != nil {
		return nil, err
	}

	booking = s.enrichBooking(ctx, booking)

	if s.publisher != nil {
		userEmail := booking.UserEmail
		if userEmail == "" && s.users != nil {
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

	if s.parking != nil && booking.SpotID > 0 {
		_ = s.parking.ReleaseSpot(ctx, strconv.FormatInt(booking.SpotID, 10))
	}

	booking = s.enrichBooking(ctx, booking)

	if s.publisher != nil {
		userEmail := booking.UserEmail
		if userEmail == "" && s.users != nil {
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

	updated, err := s.repo.UpdateStatus(ctx, id, domain.StatusActive, "")
	if err != nil {
		return nil, err
	}

	if s.parking != nil && updated.SpotID > 0 {
		_ = s.parking.UpdateSpotStatus(ctx, strconv.FormatInt(updated.SpotID, 10), "OCCUPIED")
	}

	return s.enrichBooking(ctx, updated), nil
}

func (s *BookingService) CompleteBooking(ctx context.Context, id string) (*domain.Booking, error) {
	booking, err := s.transition(ctx, id, domain.StatusCompleted, "")
	if err != nil {
		return nil, err
	}

	if s.parking != nil && booking.SpotID > 0 {
		_ = s.parking.ReleaseSpot(ctx, strconv.FormatInt(booking.SpotID, 10))
	}

	return s.enrichBooking(ctx, booking), nil
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

func (s *BookingService) enrichBooking(ctx context.Context, booking *domain.Booking) *domain.Booking {
	if booking == nil || s.users == nil {
		return booking
	}

	email, err := s.users.GetUserEmail(ctx, booking.UserID)
	if err == nil {
		booking.UserEmail = email
	}

	return booking
}

func (s *BookingService) pickRandomAvailableSpot(ctx context.Context, parkingLotID string) (int64, error) {
	spots, err := s.parking.ListParkingSpots(ctx, parkingLotID)
	if err != nil {
		return 0, err
	}

	candidates := make([]client.SpotInfo, 0, len(spots))
	for _, spot := range spots {
		if strings.EqualFold(strings.TrimSpace(spot.Status), "AVAILABLE") {
			candidates = append(candidates, spot)
		}
	}

	if len(candidates) == 0 {
		return 0, domain.ErrNoAvailableSpots
	}

	chosen := candidates[s.rng.Intn(len(candidates))]
	if chosen.ID <= 0 {
		return 0, domain.ErrInvalidInput
	}

	return chosen.ID, nil
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

func normalizeParkingError(err error) error {
	if err == nil {
		return nil
	}

	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "not found"):
		return domain.ErrParkingSpotNotFound
	case strings.Contains(msg, "not available"),
		strings.Contains(msg, "not reserved"),
		strings.Contains(msg, "already reserved"),
		strings.Contains(msg, "already occupied"),
		strings.Contains(msg, "limit reached"):
		return domain.ErrBookingConflict
	case strings.Contains(msg, "invalid"):
		return domain.ErrInvalidInput
	default:
		return fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
}
