package domain

import (
	"errors"
	"time"
)

type NotificationType string

const (
	TypeEmail NotificationType = "email"
	TypeSMS   NotificationType = "sms"
	TypePush  NotificationType = "push"
)

type NotificationStatus string

const (
	StatusPending NotificationStatus = "pending"
	StatusSent    NotificationStatus = "sent"
	StatusFailed  NotificationStatus = "failed"
)

type Notification struct {
	ID        string             `db:"id"`
	UserID    string             `db:"user_id"`
	Type      NotificationType   `db:"type"`
	Subject   string             `db:"subject"`
	Body      string             `db:"body"`
	IsRead    bool               `db:"is_read"`
	Status    NotificationStatus `db:"status"`
	CreatedAt time.Time          `db:"created_at"`
	SentAt    *time.Time         `db:"sent_at"`
}

type Preferences struct {
	UserID         string `db:"user_id"`
	EmailEnabled   bool   `db:"email_enabled"`
	SMSEnabled     bool   `db:"sms_enabled"`
	PushEnabled    bool   `db:"push_enabled"`
	MarketingEmail bool   `db:"marketing_emails"`
}

type EventBookingConfirmed struct {
	EventID   string    `json:"event_id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	BookingID string    `json:"booking_id"`
	SpotID    string    `json:"spot_id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	OccuredAt time.Time `json:"occurred_at"`
}

type EventPaymentSuccess struct {
	EventID   string    `json:"event_id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	BookingID string    `json:"booking_id"`
	Amount    int64     `json:"amount"`
	Currency  string    `json:"currency"`
	OccuredAt time.Time `json:"occurred_at"`
}

type EventBookingCancelled struct {
	EventID   string    `json:"event_id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	BookingID string    `json:"booking_id"`
	Reason    string    `json:"reason"`
	OccuredAt time.Time `json:"occurred_at"`
}

type EventUserRegistered struct {
	EventID           string    `json:"event_id"`
	UserID            string    `json:"user_id"`
	UserEmail         string    `json:"user_email"`
	FirstName         string    `json:"first_name"`
	VerificationToken string    `json:"verification_token"`
	OccuredAt         time.Time `json:"occurred_at"`
	LastName          string    `json:"last_name"`
}

type EventPasswordReset struct {
	EventID    string    `json:"event_id"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	ResetToken string    `json:"reset_token"`
	OccuredAt  time.Time `json:"occurred_at"`
}

const (
	SubjectBookingConfirmed = "parking.booking.confirmed"
	SubjectPaymentSuccess   = "parking.payment.success"
	SubjectBookingCancelled = "parking.booking.cancelled"
	SubjectUserRegistered   = "parking.user.registered"
	SubjectPasswordReset    = "parking.auth.password_reset"
)

var (
	ErrNotFound     = errors.New("notification not found")
	ErrInvalidInput = errors.New("invalid input")
)
