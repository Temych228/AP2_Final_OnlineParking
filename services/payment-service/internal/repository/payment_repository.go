package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/payment-service/internal/domain"
)

var ErrPaymentNotFound = errors.New("payment not found")

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	query := `
		INSERT INTO payments (
			id,
			booking_id,
			user_id,
			parking_id,
			spot_id,
			amount,
			method,
			status,
			provider_payment_id,
			failure_reason,
			created_at,
			paid_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		payment.ID,
		payment.BookingID,
		payment.UserID,
		payment.ParkingID,
		payment.SpotID,
		payment.Amount,
		string(payment.Method),
		string(payment.Status),
		payment.ProviderPaymentID,
		payment.FailureReason,
		payment.CreatedAt,
		payment.PaidAt,
		payment.UpdatedAt,
	)

	return err
}

func (r *PaymentRepository) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	query := `
		SELECT
			id,
			booking_id,
			user_id,
			parking_id,
			spot_id,
			amount,
			method,
			status,
			COALESCE(provider_payment_id, ''),
			failure_reason,
			created_at,
			paid_at,
			updated_at
		FROM payments
		WHERE id = $1
	`

	return r.scanPayment(r.db.QueryRowContext(ctx, query, id))
}

func (r *PaymentRepository) GetByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error) {
	query := `
		SELECT
			id,
			booking_id,
			user_id,
			parking_id,
			spot_id,
			amount,
			method,
			status,
			COALESCE(provider_payment_id, ''),
			failure_reason,
			created_at,
			paid_at,
			updated_at
		FROM payments
		WHERE booking_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	return r.scanPayment(r.db.QueryRowContext(ctx, query, bookingID))
}

func (r *PaymentRepository) List(ctx context.Context, filter domain.ListPaymentsFilter) ([]domain.Payment, error) {
	query := `
		SELECT
			id,
			booking_id,
			user_id,
			parking_id,
			spot_id,
			amount,
			method,
			status,
			COALESCE(provider_payment_id, ''),
			failure_reason,
			created_at,
			paid_at,
			updated_at
		FROM payments
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, filter.UserID, filter.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	payments := make([]domain.Payment, 0)

	for rows.Next() {
		var payment domain.Payment

		err := rows.Scan(
			&payment.ID,
			&payment.BookingID,
			&payment.UserID,
			&payment.ParkingID,
			&payment.SpotID,
			&payment.Amount,
			&payment.Method,
			&payment.Status,
			&payment.ProviderPaymentID,
			&payment.FailureReason,
			&payment.CreatedAt,
			&payment.PaidAt,
			&payment.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		payments = append(payments, payment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return payments, nil
}

func (r *PaymentRepository) MarkPaid(ctx context.Context, id string, providerPaymentID string) (*domain.Payment, error) {
	now := time.Now()

	query := `
		UPDATE payments
		SET
			status = $1,
			provider_payment_id = $2,
			paid_at = $3,
			updated_at = $3
		WHERE id = $4
		RETURNING
			id,
			booking_id,
			user_id,
			parking_id,
			spot_id,
			amount,
			method,
			status,
			COALESCE(provider_payment_id, ''),
			failure_reason,
			created_at,
			paid_at,
			updated_at
	`

	return r.scanPayment(
		r.db.QueryRowContext(
			ctx,
			query,
			string(domain.StatusPaid),
			providerPaymentID,
			now,
			id,
		),
	)
}

func (r *PaymentRepository) MarkFailed(ctx context.Context, id string, reason string) (*domain.Payment, error) {
	now := time.Now()

	query := `
		UPDATE payments
		SET
			status = $1,
			failure_reason = $2,
			updated_at = $3
		WHERE id = $4
		RETURNING
			id,
			booking_id,
			user_id,
			parking_id,
			spot_id,
			amount,
			method,
			status,
			COALESCE(provider_payment_id, ''),
			failure_reason,
			created_at,
			paid_at,
			updated_at
	`

	return r.scanPayment(
		r.db.QueryRowContext(
			ctx,
			query,
			string(domain.StatusFailed),
			reason,
			now,
			id,
		),
	)
}

func (r *PaymentRepository) Cancel(ctx context.Context, id string) (*domain.Payment, error) {
	now := time.Now()

	query := `
		UPDATE payments
		SET
			status = $1,
			updated_at = $2
		WHERE id = $3
		  AND status = $4
		RETURNING
			id,
			booking_id,
			user_id,
			parking_id,
			spot_id,
			amount,
			method,
			status,
			COALESCE(provider_payment_id, ''),
			failure_reason,
			created_at,
			paid_at,
			updated_at
	`

	return r.scanPayment(
		r.db.QueryRowContext(
			ctx,
			query,
			string(domain.StatusCancelled),
			now,
			id,
			string(domain.StatusPending),
		),
	)
}

func (r *PaymentRepository) HasPaidPaymentForBooking(ctx context.Context, bookingID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM payments
			WHERE booking_id = $1
			  AND status = $2
		)
	`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, bookingID, string(domain.StatusPaid)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *PaymentRepository) scanPayment(row rowScanner) (*domain.Payment, error) {
	var payment domain.Payment

	err := row.Scan(
		&payment.ID,
		&payment.BookingID,
		&payment.UserID,
		&payment.ParkingID,
		&payment.SpotID,
		&payment.Amount,
		&payment.Method,
		&payment.Status,
		&payment.ProviderPaymentID,
		&payment.FailureReason,
		&payment.CreatedAt,
		&payment.PaidAt,
		&payment.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}

		return nil, err
	}

	return &payment, nil
}
