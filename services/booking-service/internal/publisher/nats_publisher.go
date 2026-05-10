package publisher

import (
	"encoding/json"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/booking-service/internal/domain"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const SubjectBookingConfirmed = "parking.booking.confirmed"
const SubjectBookingCancelled = "parking.booking.cancelled"

type NATSPublisher struct{ conn *nats.Conn }

func New(conn *nats.Conn) *NATSPublisher { return &NATSPublisher{conn: conn} }

type BookingConfirmedEvent struct {
	EventID    string    `json:"event_id"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	BookingID  string    `json:"booking_id"`
	SpotID     int64     `json:"spot_id"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	OccurredAt time.Time `json:"occurred_at"`
}

type BookingCancelledEvent struct {
	EventID    string    `json:"event_id"`
	UserID     string    `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	BookingID  string    `json:"booking_id"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (p *NATSPublisher) PublishBookingConfirmed(b *domain.Booking, userEmail string) error {
	event := BookingConfirmedEvent{
		EventID:    uuid.NewString(),
		UserID:     b.UserID,
		UserEmail:  userEmail,
		BookingID:  b.ID,
		SpotID:     b.SpotID,
		StartsAt:   b.StartTime,
		EndsAt:     b.EndTime,
		OccurredAt: time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.conn.Publish(SubjectBookingConfirmed, data)
}

func (p *NATSPublisher) PublishBookingCancelled(b *domain.Booking, userEmail, reason string) error {
	event := BookingCancelledEvent{
		EventID:    uuid.NewString(),
		UserID:     b.UserID,
		UserEmail:  userEmail,
		BookingID:  b.ID,
		Reason:     reason,
		OccurredAt: time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.conn.Publish(SubjectBookingCancelled, data)
}
