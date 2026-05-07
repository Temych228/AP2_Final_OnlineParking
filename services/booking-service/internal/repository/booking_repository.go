package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const cacheKeyID = "booking:id:"

type BookingRepository struct {
	db    *pgxpool.Pool
	cache *redis.Client
	ttl   time.Duration
}

func NewBookingRepository(db *pgxpool.Pool, cache *redis.Client, ttl time.Duration) *BookingRepository {
	return &BookingRepository{db: db, cache: cache, ttl: ttl}
}

func (r *BookingRepository) Create(ctx context.Context, input domain.CreateInput) (*domain.Booking, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var hasConflict bool
	conflictQuery := `
		SELECT EXISTS (
			SELECT 1
			FROM bookings
			WHERE spot_id = $1
				AND status IN ('pending', 'confirmed', 'active')
				AND NOT ($3 <= start_time OR $2 >= end_time)
		)
	`
	if err := tx.QueryRow(ctx, conflictQuery, input.SpotID, input.StartTime, input.EndTime).Scan(&hasConflict); err != nil {
		return nil, fmt.Errorf("check booking conflict: %w", err)
	}
	if hasConflict {
		return nil, domain.ErrBookingConflict
	}

	query := `
		INSERT INTO bookings (user_id, parking_id, spot_id, vehicle_plate, start_time, end_time, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, parking_id, spot_id, vehicle_plate, start_time, end_time, status, cancel_reason, created_at, updated_at, cancelled_at
	`
	row := tx.QueryRow(
		ctx,
		query,
		input.UserID,
		input.ParkingID,
		input.SpotID,
		input.VehiclePlate,
		input.StartTime,
		input.EndTime,
		domain.StatusPending,
	)

	booking, err := scanBooking(row)
	if err != nil {
		return nil, fmt.Errorf("insert booking: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	r.setCache(ctx, booking)
	return booking, nil
}

func (r *BookingRepository) GetByID(ctx context.Context, id string) (*domain.Booking, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.ErrInvalidInput
	}

	if booking, err := r.getFromCache(ctx, cacheKeyID+id); err == nil {
		return booking, nil
	}

	query := `
		SELECT id, user_id, parking_id, spot_id, vehicle_plate, start_time, end_time, status, cancel_reason, created_at, updated_at, cancelled_at
		FROM bookings
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)

	booking, err := scanBooking(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("get booking by id: %w", err)
	}

	r.setCache(ctx, booking)
	return booking, nil
}

func (r *BookingRepository) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Booking, int, error) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)

	if filter.UserID != "" {
		args = append(args, filter.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if filter.ParkingID > 0 {
		args = append(args, filter.ParkingID)
		conditions = append(conditions, fmt.Sprintf("parking_id = $%d", len(args)))
	}
	if filter.SpotID > 0 {
		args = append(args, filter.SpotID)
		conditions = append(conditions, fmt.Sprintf("spot_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}

	whereClause := "TRUE"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM bookings WHERE " + whereClause
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count bookings: %w", err)
	}

	offset := (filter.Page - 1) * filter.PageSize
	listArgs := append(args, filter.PageSize, offset)
	listQuery := fmt.Sprintf(`
		SELECT id, user_id, parking_id, spot_id, vehicle_plate, start_time, end_time, status, cancel_reason, created_at, updated_at, cancelled_at
		FROM bookings
		WHERE %s
		ORDER BY start_time DESC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)+1, len(args)+2)

	rows, err := r.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list bookings: %w", err)
	}
	defer rows.Close()

	bookings := make([]*domain.Booking, 0)
	for rows.Next() {
		booking, err := scanBooking(rows)
		if err != nil {
			return nil, 0, err
		}
		bookings = append(bookings, booking)
	}

	return bookings, total, rows.Err()
}

func (r *BookingRepository) UpdateStatus(ctx context.Context, id string, status domain.BookingStatus, cancelReason string) (*domain.Booking, error) {
	var cancelledAt *time.Time
	if status == domain.StatusCancelled {
		now := time.Now().UTC()
		cancelledAt = &now
	}

	query := `
		UPDATE bookings
		SET status = $2, cancel_reason = $3, cancelled_at = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, parking_id, spot_id, vehicle_plate, start_time, end_time, status, cancel_reason, created_at, updated_at, cancelled_at
	`
	row := r.db.QueryRow(ctx, query, id, status, cancelReason, cancelledAt)

	booking, err := scanBooking(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("update booking status: %w", err)
	}

	r.invalidateCache(ctx, id)
	r.setCache(ctx, booking)
	return booking, nil
}

func (r *BookingRepository) getFromCache(ctx context.Context, key string) (*domain.Booking, error) {
	if r.cache == nil {
		return nil, redis.Nil
	}

	data, err := r.cache.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var booking domain.Booking
	if err := json.Unmarshal(data, &booking); err != nil {
		return nil, err
	}

	return &booking, nil
}

func (r *BookingRepository) setCache(ctx context.Context, booking *domain.Booking) {
	if r.cache == nil || booking == nil {
		return
	}

	data, err := json.Marshal(booking)
	if err != nil {
		log.Printf("cache marshal error: %v", err)
		return
	}

	if err := r.cache.Set(ctx, cacheKeyID+booking.ID, data, r.ttl).Err(); err != nil {
		log.Printf("cache set error: %v", err)
	}
}

func (r *BookingRepository) invalidateCache(ctx context.Context, id string) {
	if r.cache == nil {
		return
	}

	_, _ = r.cache.Del(ctx, cacheKeyID+id).Result()
}

func scanBooking(row interface {
	Scan(dest ...any) error
}) (*domain.Booking, error) {
	var booking domain.Booking
	err := row.Scan(
		&booking.ID,
		&booking.UserID,
		&booking.ParkingID,
		&booking.SpotID,
		&booking.VehiclePlate,
		&booking.StartTime,
		&booking.EndTime,
		&booking.Status,
		&booking.CancelReason,
		&booking.CreatedAt,
		&booking.UpdatedAt,
		&booking.CancelledAt,
	)
	if err != nil {
		return nil, err
	}

	return &booking, nil
}
