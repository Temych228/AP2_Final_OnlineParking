package publisher

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

const SubjectUserRegistered = "parking.user.registered"

type NATSPublisher struct{ conn *nats.Conn }

func New(conn *nats.Conn) *NATSPublisher { return &NATSPublisher{conn: conn} }

type UserRegisteredEvent struct {
	EventID           string    `json:"event_id"`
	UserID            string    `json:"user_id"`
	UserEmail         string    `json:"user_email"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	Phone             string    `json:"phone"`
	VerificationToken string    `json:"verification_token"`
	OccurredAt        time.Time `json:"occurred_at"`
}

func (p *NATSPublisher) PublishUserRegistered(userID, email, firstName, lastName, phone, verifyToken string) error {
	event := UserRegisteredEvent{
		EventID:           uuid.NewString(),
		UserID:            userID,
		UserEmail:         email,
		FirstName:         firstName,
		LastName:          lastName,
		Phone:             phone,
		VerificationToken: verifyToken,
		OccurredAt:        time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.conn.Publish(SubjectUserRegistered, data)
}
