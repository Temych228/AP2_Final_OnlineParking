package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/repository"
)

type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[chan *domain.Notification]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[chan *domain.Notification]struct{})}
}

func (h *Hub) Subscribe(userID string) chan *domain.Notification {
	ch := make(chan *domain.Notification, 32)

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.subs[userID]; !ok {
		h.subs[userID] = make(map[chan *domain.Notification]struct{})
	}
	h.subs[userID][ch] = struct{}{}

	return ch
}

func (h *Hub) Unsubscribe(userID string, ch chan *domain.Notification) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.subs[userID]; ok {
		delete(subs, ch)
		close(ch)
		if len(subs) == 0 {
			delete(h.subs, userID)
		}
	}
}

func (h *Hub) Broadcast(n *domain.Notification) {
	if n == nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subs[n.UserID] {
		select {
		case ch <- n:
		default:
		}
	}
}

type NotificationService struct {
	cfg  *config.Config
	repo *repository.NotificationRepository
	hub  *Hub
}

func New(cfg *config.Config, repo *repository.NotificationRepository, hub *Hub) *NotificationService {
	return &NotificationService{
		cfg:  cfg,
		repo: repo,
		hub:  hub,
	}
}

func (s *NotificationService) SendEmail(ctx context.Context, to, subject, body, userID, notifType string) (*domain.Notification, error) {
	return s.createAndDeliver(ctx, userID, to, domain.NotificationType(notifType), subject, body, true)
}

func (s *NotificationService) SendBulkEmail(ctx context.Context, recipients []string, subject, body, notifType string) (int32, int32, error) {
	var sentCount int32
	var failedCount int32

	for _, to := range recipients {
		n, err := s.createAndDeliver(ctx, "", to, domain.TypeEmail, subject, body, true)
		if err != nil || n == nil || n.Status == domain.StatusFailed {
			failedCount++
			continue
		}
		sentCount++
	}

	return sentCount, failedCount, nil
}

func (s *NotificationService) GetNotificationHistory(ctx context.Context, userID string, page, pageSize int) ([]*domain.Notification, int, error) {
	return s.repo.ListHistory(ctx, userID, page, pageSize)
}

func (s *NotificationService) MarkNotificationRead(ctx context.Context, notificationID, userID string) error {
	return s.repo.MarkRead(ctx, notificationID, userID)
}

func (s *NotificationService) GetUnreadCount(ctx context.Context, userID string) (int32, error) {
	return s.repo.UnreadCount(ctx, userID)
}

func (s *NotificationService) DeleteNotification(ctx context.Context, notificationID, userID string) error {
	return s.repo.Delete(ctx, notificationID, userID)
}

func (s *NotificationService) SendSMS(ctx context.Context, phone, message, userID string) (*domain.Notification, error) {
	subject := "SMS notification"
	body := fmt.Sprintf("Phone: %s\nMessage: %s", phone, message)
	return s.createAndDeliver(ctx, userID, "", domain.TypeSMS, subject, body, false)
}

func (s *NotificationService) SendPush(ctx context.Context, userID, title, body, data string) (*domain.Notification, error) {
	text := title + "\n" + body
	if strings.TrimSpace(data) != "" {
		text += "\n" + data
	}
	return s.createAndDeliver(ctx, userID, "", domain.TypePush, title, text, false)
}

func (s *NotificationService) GetTemplate(templateName string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(templateName)) {
	case "booking_confirmed":
		return "Booking confirmed", "Your booking has been confirmed."
	case "payment_success":
		return "Payment successful", "Your payment was processed successfully."
	case "booking_cancelled":
		return "Booking cancelled", "Your booking has been cancelled."
	case "verification_email":
		return "Verify your email", "Please verify your email address."
	case "password_reset":
		return "Reset your password", "Use the password reset link sent to you."
	default:
		return "", ""
	}
}

func (s *NotificationService) UpdatePreferences(ctx context.Context, p *domain.Preferences) error {
	if p == nil || strings.TrimSpace(p.UserID) == "" {
		return domain.ErrInvalidInput
	}
	return s.repo.UpsertPreferences(ctx, p)
}

func (s *NotificationService) GetPreferences(ctx context.Context, userID string) (*domain.Preferences, error) {
	return s.repo.GetPreferences(ctx, userID)
}

func (s *NotificationService) HandleEvent(ctx context.Context, subject string, payload []byte) error {
	ok, err := s.repo.MarkEventProcessed(ctx, subject+":"+string(payload), 24*time.Hour)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	switch subject {
	case domain.SubjectUserRegistered:
		return s.handleUserRegistered(ctx, payload)
	case domain.SubjectPasswordReset:
		return s.handlePasswordReset(ctx, payload)
	case domain.SubjectBookingConfirmed:
		return s.handleBookingConfirmed(ctx, payload)
	case domain.SubjectPaymentSuccess:
		return s.handlePaymentSuccess(ctx, payload)
	case domain.SubjectBookingCancelled:
		return s.handleBookingCancelled(ctx, payload)
	default:
		return nil
	}
}

func (s *NotificationService) createAndDeliver(ctx context.Context, userID, to string, typ domain.NotificationType, subject, body string, emailDelivery bool) (*domain.Notification, error) {
	n, err := s.repo.Create(ctx, &domain.Notification{
		UserID:  userID,
		Type:    typ,
		Subject: subject,
		Body:    body,
		IsRead:  false,
		Status:  domain.StatusPending,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	if emailDelivery && typ == domain.TypeEmail {
		if err := s.sendSMTP(to, subject, body); err != nil {
			_ = s.repo.UpdateStatus(ctx, n.ID, domain.StatusFailed, nil)
			n.Status = domain.StatusFailed
			s.hub.Broadcast(n)
			return n, err
		}
		_ = s.repo.UpdateStatus(ctx, n.ID, domain.StatusSent, &now)
		n.Status = domain.StatusSent
		n.SentAt = &now
		s.hub.Broadcast(n)
		return n, nil
	}

	_ = s.repo.UpdateStatus(ctx, n.ID, domain.StatusSent, &now)
	n.Status = domain.StatusSent
	n.SentAt = &now
	s.hub.Broadcast(n)
	return n, nil
}

func (s *NotificationService) sendSMTP(to, subject, body string) error {
	if strings.TrimSpace(s.cfg.SMTPHost) == "" || strings.TrimSpace(s.cfg.SMTPFrom) == "" {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	auth := smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)

	msg := []byte(
		"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
			body,
	)

	return smtp.SendMail(addr, auth, s.cfg.SMTPFrom, []string{to}, msg)
}

func (s *NotificationService) handleUserRegistered(ctx context.Context, payload []byte) error {
	var ev domain.EventUserRegistered
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	link := strings.TrimRight(s.cfg.FrontendURL, "/") + "/verify-email?token=" + ev.VerificationToken
	subject := "Verify your email"
	body := fmt.Sprintf("Hello %s,\n\nPlease verify your email using this link:\n%s\n", ev.FirstName, link)

	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func (s *NotificationService) handlePasswordReset(ctx context.Context, payload []byte) error {
	var ev domain.EventPasswordReset
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	link := strings.TrimRight(s.cfg.FrontendURL, "/") + "/reset-password?token=" + ev.ResetToken
	subject := "Reset your password"
	body := fmt.Sprintf("Use this link to reset your password:\n%s\n", link)

	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func (s *NotificationService) handleBookingConfirmed(ctx context.Context, payload []byte) error {
	var ev domain.EventBookingConfirmed
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	subject := "Booking confirmed"
	body := fmt.Sprintf("Booking %s for spot %s is confirmed.", ev.BookingID, ev.SpotID)

	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func (s *NotificationService) handlePaymentSuccess(ctx context.Context, payload []byte) error {
	var ev domain.EventPaymentSuccess
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	subject := "Payment successful"
	body := fmt.Sprintf("Payment for booking %s was successful. Amount: %d %s.", ev.BookingID, ev.Amount, ev.Currency)

	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func (s *NotificationService) handleBookingCancelled(ctx context.Context, payload []byte) error {
	var ev domain.EventBookingCancelled
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	subject := "Booking cancelled"
	body := fmt.Sprintf("Booking %s was cancelled. Reason: %s", ev.BookingID, ev.Reason)

	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func jsonUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *NotificationService) Subscribe(userID string) chan *domain.Notification {
	return s.hub.Subscribe(userID)
}

func (s *NotificationService) Unsubscribe(userID string, ch chan *domain.Notification) {
	s.hub.Unsubscribe(userID, ch)
}
