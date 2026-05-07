package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type NotificationRepository struct {
	db    *pgxpool.Pool
	cache *redis.Client
}

func New(db *pgxpool.Pool, cache *redis.Client) *NotificationRepository {
	return &NotificationRepository{db: db, cache: cache}
}

func (r *NotificationRepository) Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO notifications (user_id, type, subject, body, is_read, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, type, subject, body, is_read, status, created_at, sent_at
	`, n.UserID, n.Type, n.Subject, n.Body, n.IsRead, n.Status)

	return scanNotification(row)
}

func (r *NotificationRepository) UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus, sentAt *time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET status = $2, sent_at = $3
		WHERE id = $1
	`, id, status, sentAt)
	return err
}

func (r *NotificationRepository) ListHistory(ctx context.Context, userID string, page, pageSize int) ([]*domain.Notification, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1
	`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, type, subject, body, is_read, status, created_at, sent_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, n)
	}

	return items, total, rows.Err()
}

func (r *NotificationRepository) MarkRead(ctx context.Context, notificationID, userID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET is_read = true
		WHERE id = $1 AND user_id = $2
	`, notificationID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) Delete(ctx context.Context, notificationID, userID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM notifications
		WHERE id = $1 AND user_id = $2
	`, notificationID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) UnreadCount(ctx context.Context, userID string) (int32, error) {
	var count int32
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1 AND is_read = false
	`, userID).Scan(&count)
	return count, err
}

func (r *NotificationRepository) GetPreferences(ctx context.Context, userID string) (*domain.Preferences, error) {
	row := r.db.QueryRow(ctx, `
		SELECT user_id, email_enabled, sms_enabled, push_enabled, marketing_emails
		FROM notification_preferences
		WHERE user_id = $1
	`, userID)

	var p domain.Preferences
	err := row.Scan(&p.UserID, &p.EmailEnabled, &p.SMSEnabled, &p.PushEnabled, &p.MarketingEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.Preferences{
				UserID:         userID,
				EmailEnabled:   true,
				SMSEnabled:     false,
				PushEnabled:    true,
				MarketingEmail: false,
			}, nil
		}
		return nil, err
	}

	return &p, nil
}

func (r *NotificationRepository) UpsertPreferences(ctx context.Context, p *domain.Preferences) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notification_preferences (user_id, email_enabled, sms_enabled, push_enabled, marketing_emails)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET email_enabled = EXCLUDED.email_enabled,
			sms_enabled = EXCLUDED.sms_enabled,
			push_enabled = EXCLUDED.push_enabled,
			marketing_emails = EXCLUDED.marketing_emails,
			updated_at = NOW()
	`, p.UserID, p.EmailEnabled, p.SMSEnabled, p.PushEnabled, p.MarketingEmail)
	return err
}

func (r *NotificationRepository) MarkEventProcessed(ctx context.Context, eventID string, ttl time.Duration) (bool, error) {
	if r.cache == nil {
		return true, nil
	}

	key := "notification:event:" + strings.TrimSpace(eventID)
	ok, err := r.cache.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func scanNotification(row interface{ Scan(dest ...any) error }) (*domain.Notification, error) {
	var (
		n       domain.Notification
		sentAt  sql.NullTime
		created sql.NullTime
	)

	err := row.Scan(
		&n.ID,
		&n.UserID,
		&n.Type,
		&n.Subject,
		&n.Body,
		&n.IsRead,
		&n.Status,
		&created,
		&sentAt,
	)
	if err != nil {
		return nil, err
	}

	if created.Valid {
		n.CreatedAt = created.Time
	}

	if sentAt.Valid {
		t := sentAt.Time
		n.SentAt = &t
	}

	return &n, nil
}

func normalizeJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func toSubjectType(s string) domain.NotificationType {
	switch s {
	case "sms":
		return domain.TypeSMS
	case "push":
		return domain.TypePush
	default:
		return domain.TypeEmail
	}
}

func (r *NotificationRepository) DB() *pgxpool.Pool {
	return r.db
}

func (r *NotificationRepository) Cache() *redis.Client {
	return r.cache
}
