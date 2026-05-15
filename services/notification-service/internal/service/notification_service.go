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

type NotificationRepo interface {
	Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error)
	UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus, sentAt *time.Time) error
	ListHistory(ctx context.Context, userID string, page, pageSize int) ([]*domain.Notification, int, error)
	MarkRead(ctx context.Context, notificationID, userID string) error
	Delete(ctx context.Context, notificationID, userID string) error
	UnreadCount(ctx context.Context, userID string) (int32, error)
	GetPreferences(ctx context.Context, userID string) (*domain.Preferences, error)
	UpsertPreferences(ctx context.Context, p *domain.Preferences) error
	MarkEventProcessed(ctx context.Context, eventID string, ttl time.Duration) (bool, error)
}

type NotificationService struct {
	cfg  *config.Config
	repo NotificationRepo
	hub  *Hub
}

func New(cfg *config.Config, repo NotificationRepo, hub *Hub) *NotificationService {
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
		return fmt.Errorf("smtp is not configured")
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	auth := smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)

	msg := []byte(
		"From: " + s.cfg.SMTPFrom + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"Date: " + time.Now().Format(time.RFC1123Z) + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n" +
			"\r\n" +
			body + "\r\n",
	)

	if err := smtp.SendMail(addr, auth, s.cfg.SMTPFrom, []string{to}, msg); err != nil {
		return fmt.Errorf("smtp send failed: %w", err)
	}

	return nil
}

func emailLayout(accentColor, icon, title, preheader, content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title></head>
<body style="margin:0;padding:0;background:#f4f6f9;font-family:Arial,sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f6f9;padding:40px 0;">
    <tr><td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.08);">
        <!-- Header -->
        <tr><td style="background:%s;padding:36px 40px;text-align:center;">
          <div style="font-size:48px;margin-bottom:12px;">%s</div>
          <h1 style="color:#ffffff;margin:0;font-size:26px;font-weight:700;letter-spacing:-0.5px;">%s</h1>
        </td></tr>
        <!-- Body -->
        <tr><td style="padding:40px;">
          %s
        </td></tr>
        <!-- Footer -->
        <tr><td style="background:#f4f6f9;padding:24px 40px;text-align:center;border-top:1px solid #e8ecf0;">
          <p style="margin:0;color:#9ca3af;font-size:13px;">© 2025 Online Parking. All rights reserved.</p>
          <p style="margin:6px 0 0;color:#9ca3af;font-size:12px;">This is an automated message, please do not reply.</p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, preheader, accentColor, icon, title, content)
}

func btnPrimary(color, link, text string) string {
	return fmt.Sprintf(`<div style="text-align:center;margin:32px 0;">
    <a href="%s" style="display:inline-block;background:%s;color:#fff;text-decoration:none;padding:14px 36px;border-radius:8px;font-size:16px;font-weight:700;letter-spacing:0.3px;">%s</a>
  </div>`, link, color, text)
}

func infoBox(rows [][2]string) string {
	var sb strings.Builder
	sb.WriteString(`<table width="100%%" cellpadding="0" cellspacing="0" style="background:#f8fafc;border-radius:8px;border:1px solid #e2e8f0;margin:24px 0;">`)
	for _, row := range rows {
		fmt.Fprintf(&sb, `<tr>
      <td style="padding:12px 20px;color:#6b7280;font-size:14px;width:40%%;border-bottom:1px solid #e2e8f0;">%s</td>
      <td style="padding:12px 20px;color:#111827;font-size:14px;font-weight:600;border-bottom:1px solid #e2e8f0;">%s</td>
    </tr>`, row[0], row[1])
	}
	sb.WriteString(`</table>`)
	return sb.String()
}

func (s *NotificationService) handleUserRegistered(ctx context.Context, payload []byte) error {
	var ev domain.EventUserRegistered
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	link := strings.TrimRight(s.cfg.FrontendURL, "/") + "/verify-email?token=" + ev.VerificationToken
	subject := "Welcome to Online Parking! Verify your email"

	content := fmt.Sprintf(`
    <p style="color:#374151;font-size:16px;line-height:1.6;margin:0 0 16px;">Hi <strong>%s</strong>, welcome aboard! 🎉</p>
    <p style="color:#374151;font-size:15px;line-height:1.6;margin:0 0 8px;">Your account has been successfully created. Please verify your email address to get started.</p>
    %s
    %s
    <p style="color:#6b7280;font-size:13px;margin:24px 0 0;">This link expires in 24 hours. If you didn't create this account, you can safely ignore this email.</p>`,
		ev.FirstName,
		infoBox([][2]string{
			{"Full name", ev.FirstName + " " + ev.LastName},
			{"Email", ev.UserEmail},
			{"Registered at", time.Now().Format("02 Jan 2006, 15:04")},
		}),
		btnPrimary("#4f46e5", link, "Verify Email Address"),
	)

	body := emailLayout("#4f46e5", "🅿️", "Account Created!", subject, content)
	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func (s *NotificationService) handlePasswordReset(ctx context.Context, payload []byte) error {
	var ev domain.EventPasswordReset
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	link := strings.TrimRight(s.cfg.FrontendURL, "/") + "/reset-password?token=" + ev.ResetToken
	subject := "Reset your password — Online Parking"

	content := fmt.Sprintf(`
    <p style="color:#374151;font-size:16px;line-height:1.6;margin:0 0 16px;">We received a request to reset your password.</p>
    <p style="color:#374151;font-size:15px;line-height:1.6;margin:0 0 8px;">Click the button below to create a new password. This link is valid for <strong>1 hour</strong>.</p>
    %s
    <p style="color:#6b7280;font-size:13px;margin:24px 0 0;">If you didn't request a password reset, please ignore this email. Your password will remain unchanged.</p>`,
		btnPrimary("#dc2626", link, "Reset Password"),
	)

	body := emailLayout("#dc2626", "🔐", "Password Reset", subject, content)
	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func (s *NotificationService) handleBookingConfirmed(ctx context.Context, payload []byte) error {
	var ev domain.EventBookingConfirmed
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	subject := "Your parking spot is booked!"

	content := fmt.Sprintf(`
    <p style="color:#374151;font-size:16px;line-height:1.6;margin:0 0 16px;">Great news! Your parking spot has been <strong>successfully reserved</strong>.</p>
    %s
    <p style="color:#6b7280;font-size:13px;margin:24px 0 0;">Please arrive on time. Late arrivals may result in spot reallocation.</p>`,
		infoBox([][2]string{
			{"Booking ID", ev.BookingID},
			{"Spot ID", ev.SpotID},
			{"Start time", ev.StartsAt.Format("02 Jan 2006, 15:04")},
			{"End time", ev.EndsAt.Format("02 Jan 2006, 15:04")},
			{"Status", "Confirmed"},
		}),
	)

	body := emailLayout("#059669", "🅿️", "Booking Confirmed", subject, content)
	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func (s *NotificationService) handlePaymentSuccess(ctx context.Context, payload []byte) error {
	var ev domain.EventPaymentSuccess
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	subject := "Payment successful — Online Parking"

	content := fmt.Sprintf(`
    <p style="color:#374151;font-size:16px;line-height:1.6;margin:0 0 16px;">Your payment has been <strong>processed successfully</strong>. Thank you!</p>
    %s
    <p style="color:#6b7280;font-size:13px;margin:24px 0 0;">Keep this email as your payment receipt. For questions, contact our support.</p>`,
		infoBox([][2]string{
			{"Booking ID", ev.BookingID},
			{"Amount", fmt.Sprintf("%d %s", ev.Amount, ev.Currency)},
			{"Date", time.Now().Format("02 Jan 2006, 15:04")},
			{"Status", "Paid"},
		}),
	)

	body := emailLayout("#0284c7", "✅", "Payment Successful", subject, content)
	_, err := s.createAndDeliver(ctx, ev.UserID, ev.UserEmail, domain.TypeEmail, subject, body, true)
	return err
}

func (s *NotificationService) handleBookingCancelled(ctx context.Context, payload []byte) error {
	var ev domain.EventBookingCancelled
	if err := jsonUnmarshal(payload, &ev); err != nil {
		return err
	}

	subject := "Your booking has been cancelled — Online Parking"

	content := fmt.Sprintf(`
    <p style="color:#374151;font-size:16px;line-height:1.6;margin:0 0 16px;">Your parking booking has been <strong>cancelled</strong>.</p>
    %s
    <p style="color:#6b7280;font-size:13px;margin:24px 0 0;">If you believe this was a mistake or need assistance, please contact our support team.</p>`,
		infoBox([][2]string{
			{"Booking ID", ev.BookingID},
			{"Cancelled at", time.Now().Format("02 Jan 2006, 15:04")},
			{"Reason", ev.Reason},
			{"Status", "Cancelled"},
		}),
	)

	body := emailLayout("#b45309", "❌", "Booking Cancelled", subject, content)
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
