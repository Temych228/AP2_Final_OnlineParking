package publisher

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const SubjectPaymentSuccess = "parking.payment.success"

type NATSPublisher struct {
	conn *nats.Conn
}

func NewNATSPublisher(conn *nats.Conn) *NATSPublisher {
	return &NATSPublisher{conn: conn}
}

type PaymentSuccessEvent struct {
	EventID   string  `json:"event_id"`
	UserID    string  `json:"user_id"`
	UserEmail string  `json:"user_email"`
	BookingID string  `json:"booking_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	CreatedAt string  `json:"created_at"`
}

func (p *NATSPublisher) PublishPaymentSuccess(ctx context.Context, userID, userEmail, bookingID string, amount float64) error {
	event := PaymentSuccessEvent{
		EventID:   uuid.NewString(),
		UserID:    userID,
		UserEmail: userEmail,
		BookingID: bookingID,
		Amount:    amount,
		Currency:  "KZT",
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.conn.Publish(SubjectPaymentSuccess, data)
}
