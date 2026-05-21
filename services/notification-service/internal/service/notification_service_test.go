package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/service"
	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/transport/http"
	"github.com/gin-gonic/gin"
)

type fakeNotificationRepo struct {
	notifications map[string]*domain.Notification
	preferences   map[string]*domain.Preferences
	events        map[string]time.Time
	nextID        int
}

func newFakeNotificationRepo() *fakeNotificationRepo {
	return &fakeNotificationRepo{
		notifications: make(map[string]*domain.Notification),
		preferences:   make(map[string]*domain.Preferences),
		events:        make(map[string]time.Time),
		nextID:        1,
	}
}

func (r *fakeNotificationRepo) clone(n *domain.Notification) *domain.Notification {
	if n == nil {
		return nil
	}
	copy := *n
	return &copy
}

func (r *fakeNotificationRepo) Create(_ context.Context, notification *domain.Notification) (*domain.Notification, error) {
	id := notification.ID
	if strings.TrimSpace(id) == "" {
		id = fmt.Sprintf("notif-%d", r.nextID)
		r.nextID++
	}
	n := *notification
	n.ID = id
	n.CreatedAt = time.Now().UTC()
	r.notifications[id] = &n
	return &n, nil
}

func (r *fakeNotificationRepo) UpdateStatus(_ context.Context, id string, status domain.NotificationStatus, sentAt *time.Time) error {
	n, ok := r.notifications[strings.TrimSpace(id)]
	if !ok {
		return domain.ErrNotFound
	}
	n.Status = status
	n.SentAt = sentAt
	return nil
}

func (r *fakeNotificationRepo) ListHistory(_ context.Context, userID string, page, pageSize int) ([]*domain.Notification, int, error) {
	items := make([]*domain.Notification, 0)
	for _, n := range r.notifications {
		if n.UserID == strings.TrimSpace(userID) {
			items = append(items, r.clone(n))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, len(items), nil
}

func (r *fakeNotificationRepo) MarkRead(_ context.Context, notificationID, userID string) error {
	n, ok := r.notifications[strings.TrimSpace(notificationID)]
	if !ok || n.UserID != strings.TrimSpace(userID) {
		return domain.ErrNotFound
	}
	n.IsRead = true
	return nil
}

func (r *fakeNotificationRepo) UnreadCount(_ context.Context, userID string) (int32, error) {
	var count int32
	for _, n := range r.notifications {
		if n.UserID == strings.TrimSpace(userID) && !n.IsRead {
			count++
		}
	}
	return count, nil
}

func (r *fakeNotificationRepo) Delete(_ context.Context, notificationID, userID string) error {
	if n, ok := r.notifications[strings.TrimSpace(notificationID)]; ok && n.UserID == strings.TrimSpace(userID) {
		delete(r.notifications, strings.TrimSpace(notificationID))
		return nil
	}
	return domain.ErrNotFound
}

func (r *fakeNotificationRepo) MarkEventProcessed(_ context.Context, eventKey string, ttl time.Duration) (bool, error) {
	if _, ok := r.events[eventKey]; ok {
		return false, nil
	}
	r.events[eventKey] = time.Now().Add(ttl)
	return true, nil
}

func (r *fakeNotificationRepo) GetPreferences(_ context.Context, userID string) (*domain.Preferences, error) {
	if p, ok := r.preferences[strings.TrimSpace(userID)]; ok {
		copy := *p
		return &copy, nil
	}
	return &domain.Preferences{
		UserID:         strings.TrimSpace(userID),
		EmailEnabled:   true,
		SMSEnabled:     false,
		PushEnabled:    true,
		MarketingEmail: false,
	}, nil
}

func (r *fakeNotificationRepo) UpsertPreferences(_ context.Context, p *domain.Preferences) error {
	copy := *p
	r.preferences[strings.TrimSpace(p.UserID)] = &copy
	return nil
}

func newNotificationService() (*service.NotificationService, *fakeNotificationRepo) {
	repo := newFakeNotificationRepo()
	cfg := &config.Config{
		FrontendURL: "https://example.com",
		SMTPHost:    "",
		SMTPPort:    25,
		SMTPFrom:    "noreply@example.com",
	}
	return service.New(cfg, repo, service.NewHub()), repo
}

func TestNotificationService_Unit(t *testing.T) {
	svc, repo := newNotificationService()
	ctx := context.Background()

	n, err := svc.SendPush(ctx, "user-1", "Hello", "World", "meta")
	if err != nil {
		t.Fatalf("send push: %v", err)
	}
	if n.Status != domain.StatusSent {
		t.Fatalf("expected sent status, got %s", n.Status)
	}

	if count, err := svc.GetUnreadCount(ctx, "user-1"); err != nil || count != 1 {
		t.Fatalf("unexpected unread count: %d err=%v", count, err)
	}

	if err := svc.MarkNotificationRead(ctx, n.ID, "user-1"); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if count, err := svc.GetUnreadCount(ctx, "user-1"); err != nil || count != 0 {
		t.Fatalf("unexpected unread count after read: %d err=%v", count, err)
	}

	if subject, body := svc.GetTemplate("booking_confirmed"); subject == "" || body == "" {
		t.Fatal("expected template content")
	}

	if err := svc.UpdatePreferences(ctx, &domain.Preferences{UserID: "user-1", EmailEnabled: true, SMSEnabled: true, PushEnabled: false, MarketingEmail: true}); err != nil {
		t.Fatalf("update preferences: %v", err)
	}
	pref, err := svc.GetPreferences(ctx, "user-1")
	if err != nil {
		t.Fatalf("get preferences: %v", err)
	}
	if !pref.EmailEnabled || !pref.SMSEnabled || pref.PushEnabled {
		t.Fatalf("unexpected preferences: %#v", pref)
	}

	payload, _ := json.Marshal(domain.EventUserRegistered{
		EventID:           "ev-1",
		UserID:            "user-2",
		UserEmail:         "second@mail.com",
		FirstName:         "Second",
		VerificationToken: "token-123",
		OccuredAt:         time.Now().UTC(),
	})
	if err := svc.HandleEvent(ctx, domain.SubjectUserRegistered, payload); err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if err := svc.HandleEvent(ctx, domain.SubjectUserRegistered, payload); err != nil {
		t.Fatalf("handle dedupe event: %v", err)
	}

	items, total, err := svc.GetNotificationHistory(ctx, "user-2", 1, 20)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one history item, got total=%d len=%d", total, len(items))
	}

	if _, err := svc.SendEmail(ctx, "to@example.com", "subject", "body", "user-3", string(domain.TypeEmail)); err == nil {
		t.Fatal("expected smtp config error for SendEmail")
	}

	if _, err := svc.SendSMS(ctx, "+77001112233", "hello", "user-4"); err != nil {
		t.Fatalf("send sms: %v", err)
	}

	if _, err := repo.Create(ctx, &domain.Notification{UserID: "user-1", Type: domain.TypeEmail, Subject: "s", Body: "b", Status: domain.StatusPending}); err != nil {
		t.Fatalf("seed notification: %v", err)
	}
}

func TestNotificationService_Errors(t *testing.T) {
	svc, _ := newNotificationService()
	ctx := context.Background()

	if err := svc.UpdatePreferences(ctx, nil); err == nil {
		t.Fatal("expected invalid input")
	}

	if _, err := svc.SendEmail(ctx, "to@example.com", "subject", "body", "user-1", string(domain.TypeEmail)); err == nil {
		t.Fatal("expected smtp config error")
	}
}

func TestNotificationHTTP_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newNotificationService()
	handler := httptransport.New(svc)
	mux := http.NewServeMux()
	handler.Register(mux)

	pushResp := httptest.NewRecorder()
	mux.ServeHTTP(pushResp, httptest.NewRequest(http.MethodPost, "/notifications/push", mustJSON(t, map[string]any{
		"user_id": "user-1",
		"title":   "Title",
		"body":    "Body",
		"data":    "meta",
	})))
	if pushResp.Code != http.StatusCreated {
		t.Fatalf("push status = %d body=%s", pushResp.Code, pushResp.Body.String())
	}

	var pushed map[string]any
	if err := json.Unmarshal(pushResp.Body.Bytes(), &pushed); err != nil {
		t.Fatalf("decode push response: %v", err)
	}
	notification, _ := pushed["notification"].(map[string]any)
	id, _ := notification["id"].(string)
	if id == "" {
		t.Fatalf("expected notification id, got %s", pushResp.Body.String())
	}

	historyResp := httptest.NewRecorder()
	mux.ServeHTTP(historyResp, httptest.NewRequest(http.MethodGet, "/notifications/history?user_id=user-1&page=1&page_size=20", nil))
	if historyResp.Code != http.StatusOK {
		t.Fatalf("history status = %d body=%s", historyResp.Code, historyResp.Body.String())
	}

	unreadResp := httptest.NewRecorder()
	mux.ServeHTTP(unreadResp, httptest.NewRequest(http.MethodGet, "/notifications/unread-count?user_id=user-1", nil))
	if unreadResp.Code != http.StatusOK {
		t.Fatalf("unread status = %d body=%s", unreadResp.Code, unreadResp.Body.String())
	}

	readResp := httptest.NewRecorder()
	markReadBody, _ := json.Marshal(map[string]any{"user_id": "user-1"})
	mux.ServeHTTP(readResp, httptest.NewRequest(http.MethodPost, "/notifications/"+id+"/read", bytes.NewReader(markReadBody)))
	if readResp.Code != http.StatusOK {
		t.Fatalf("read status = %d body=%s", readResp.Code, readResp.Body.String())
	}

	prefPutResp := httptest.NewRecorder()
	mux.ServeHTTP(prefPutResp, httptest.NewRequest(http.MethodPut, "/notifications/preferences", mustJSON(t, map[string]any{
		"user_id":          "user-1",
		"email_enabled":    true,
		"sms_enabled":      true,
		"push_enabled":     false,
		"marketing_emails": true,
	})))
	if prefPutResp.Code != http.StatusOK {
		t.Fatalf("preferences put status = %d body=%s", prefPutResp.Code, prefPutResp.Body.String())
	}

	prefGetResp := httptest.NewRecorder()
	mux.ServeHTTP(prefGetResp, httptest.NewRequest(http.MethodGet, "/notifications/preferences?user_id=user-1", nil))
	if prefGetResp.Code != http.StatusOK {
		t.Fatalf("preferences get status = %d body=%s", prefGetResp.Code, prefGetResp.Body.String())
	}

	templateResp := httptest.NewRecorder()
	mux.ServeHTTP(templateResp, httptest.NewRequest(http.MethodGet, "/notifications/templates/booking_confirmed", nil))
	if templateResp.Code != http.StatusOK {
		t.Fatalf("template status = %d body=%s", templateResp.Code, templateResp.Body.String())
	}

	deleteResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteResp, httptest.NewRequest(http.MethodDelete, "/notifications/"+id+"?user_id=user-1", nil))
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", deleteResp.Code, deleteResp.Body.String())
	}
}

func mustJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return bytes.NewReader(data)
}

func TestNotificationService_HandleEvent_AllSubjects(t *testing.T) {
	svc, _ := newNotificationService()
	ctx := context.Background()

	t.Run("booking_confirmed", func(t *testing.T) {
		payload, _ := json.Marshal(domain.EventBookingConfirmed{
			EventID:   "ev-bc-1",
			UserID:    "user-10",
			UserEmail: "user10@mail.com",
			BookingID: "book-1",
			SpotID:    "spot-1",
			StartsAt:  time.Now().UTC(),
			EndsAt:    time.Now().Add(2 * time.Hour).UTC(),
			OccuredAt: time.Now().UTC(),
		})
		if err := svc.HandleEvent(ctx, domain.SubjectBookingConfirmed, payload); err != nil {
			t.Fatalf("booking_confirmed: %v", err)
		}
		if err := svc.HandleEvent(ctx, domain.SubjectBookingConfirmed, payload); err != nil {
			t.Fatalf("booking_confirmed dedupe: %v", err)
		}
	})

	t.Run("payment_success", func(t *testing.T) {
		payload, _ := json.Marshal(domain.EventPaymentSuccess{
			EventID:   "ev-ps-1",
			UserID:    "user-11",
			UserEmail: "user11@mail.com",
			BookingID: "book-2",
			Amount:    5000,
			Currency:  "KZT",
			OccuredAt: time.Now().UTC(),
		})
		if err := svc.HandleEvent(ctx, domain.SubjectPaymentSuccess, payload); err != nil {
			t.Fatalf("payment_success: %v", err)
		}
	})

	t.Run("booking_cancelled", func(t *testing.T) {
		payload, _ := json.Marshal(domain.EventBookingCancelled{
			EventID:   "ev-bcancel-1",
			UserID:    "user-12",
			UserEmail: "user12@mail.com",
			BookingID: "book-3",
			Reason:    "user request",
			OccuredAt: time.Now().UTC(),
		})
		if err := svc.HandleEvent(ctx, domain.SubjectBookingCancelled, payload); err != nil {
			t.Fatalf("booking_cancelled: %v", err)
		}
	})

	t.Run("password_reset", func(t *testing.T) {
		payload, _ := json.Marshal(domain.EventPasswordReset{
			EventID:    "ev-pr-1",
			UserID:     "user-13",
			UserEmail:  "user13@mail.com",
			ResetToken: "reset-token-abc",
			OccuredAt:  time.Now().UTC(),
		})
		if err := svc.HandleEvent(ctx, domain.SubjectPasswordReset, payload); err != nil {
			t.Fatalf("password_reset: %v", err)
		}
	})

	t.Run("unknown_subject", func(t *testing.T) {
		if err := svc.HandleEvent(ctx, "parking.unknown.event", []byte(`{}`)); err != nil {
			t.Fatalf("unknown subject should not return error: %v", err)
		}
	})

	t.Run("invalid_json_booking_confirmed", func(t *testing.T) {
		err := svc.HandleEvent(ctx, domain.SubjectBookingConfirmed, []byte(`not-json`))
		if err == nil {
			t.Fatal("expected error on invalid JSON")
		}
	})

	t.Run("invalid_json_payment_success", func(t *testing.T) {
		err := svc.HandleEvent(ctx, domain.SubjectPaymentSuccess, []byte(`{bad}`))
		if err == nil {
			t.Fatal("expected error on invalid JSON for payment_success")
		}
	})

	t.Run("invalid_json_booking_cancelled", func(t *testing.T) {
		err := svc.HandleEvent(ctx, domain.SubjectBookingCancelled, []byte(`{bad}`))
		if err == nil {
			t.Fatal("expected error on invalid JSON for booking_cancelled")
		}
	})

	t.Run("invalid_json_password_reset", func(t *testing.T) {
		err := svc.HandleEvent(ctx, domain.SubjectPasswordReset, []byte(`{bad}`))
		if err == nil {
			t.Fatal("expected error on invalid JSON for password_reset")
		}
	})

	t.Run("invalid_json_user_registered", func(t *testing.T) {
		err := svc.HandleEvent(ctx, domain.SubjectUserRegistered, []byte(`{bad}`))
		if err == nil {
			t.Fatal("expected error on invalid JSON for user_registered")
		}
	})
}

func TestNotificationService_Hub(t *testing.T) {
	hub := service.NewHub()
	ctx := context.Background()
	_ = ctx

	ch1 := hub.Subscribe("user-hub-1")
	ch2 := hub.Subscribe("user-hub-1")

	n := &domain.Notification{
		ID:      "hub-notif-1",
		UserID:  "user-hub-1",
		Type:    domain.TypePush,
		Subject: "test",
		Body:    "body",
		Status:  domain.StatusSent,
	}

	hub.Broadcast(n)

	select {
	case got := <-ch1:
		if got.ID != n.ID {
			t.Fatalf("ch1 got wrong notification: %+v", got)
		}
	default:
		t.Fatal("ch1 did not receive broadcast")
	}

	select {
	case got := <-ch2:
		if got.ID != n.ID {
			t.Fatalf("ch2 got wrong notification: %+v", got)
		}
	default:
		t.Fatal("ch2 did not receive broadcast")
	}

	hub.Broadcast(nil)

	hub.Unsubscribe("user-hub-1", ch1)
	hub.Unsubscribe("user-hub-1", ch2)

	hub.Broadcast(n)

	hub.Unsubscribe("nonexistent-user", ch1)
}

func TestNotificationService_SendBulkEmail(t *testing.T) {
	svc, _ := newNotificationService()
	ctx := context.Background()

	sent, failed, err := svc.SendBulkEmail(ctx,
		[]string{"a@mail.com", "b@mail.com", "c@mail.com"},
		"Bulk Subject",
		"Bulk Body",
		string(domain.TypeEmail),
	)
	if err != nil {
		t.Fatalf("SendBulkEmail unexpected error: %v", err)
	}
	if sent+failed != 3 {
		t.Fatalf("expected sent+failed=3, got sent=%d failed=%d", sent, failed)
	}
}

func TestNotificationService_DeleteNotification(t *testing.T) {
	svc, repo := newNotificationService()
	ctx := context.Background()

	n, err := svc.SendPush(ctx, "user-del-1", "Del Title", "Del Body", "")
	if err != nil {
		t.Fatalf("SendPush: %v", err)
	}

	if err := svc.DeleteNotification(ctx, n.ID, "user-del-1"); err != nil {
		t.Fatalf("DeleteNotification: %v", err)
	}

	items, total, err := svc.GetNotificationHistory(ctx, "user-del-1", 1, 20)
	if err != nil {
		t.Fatalf("GetNotificationHistory after delete: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected 0 after delete, got total=%d", total)
	}

	if err := svc.DeleteNotification(ctx, "nonexistent", "user-del-1"); err == nil {
		t.Fatal("expected error deleting nonexistent")
	}

	_ = repo
}

func TestNotificationService_GetTemplate_AllTemplates(t *testing.T) {
	svc, _ := newNotificationService()

	templates := []string{
		"booking_confirmed",
		"payment_success",
		"booking_cancelled",
		"verification_email",
		"password_reset",
		"unknown_template",
	}

	for _, name := range templates {
		subject, body := svc.GetTemplate(name)
		switch name {
		case "unknown_template":
			if subject != "" || body != "" {
				t.Fatalf("expected empty template for unknown, got subject=%q body=%q", subject, body)
			}
		default:
			if subject == "" || body == "" {
				t.Fatalf("expected non-empty template for %q", name)
			}
		}
	}
}

func TestNotificationService_SubscribeUnsubscribe(t *testing.T) {
	svc, _ := newNotificationService()

	ch := svc.Subscribe("sub-user-1")
	if ch == nil {
		t.Fatal("expected non-nil channel from Subscribe")
	}

	svc.Unsubscribe("sub-user-1", ch)
	svc.Unsubscribe("nonexistent-user", ch)
}

func TestNotificationService_MarkRead_NotFound(t *testing.T) {
	svc, _ := newNotificationService()
	ctx := context.Background()

	err := svc.MarkNotificationRead(ctx, "does-not-exist", "some-user")
	if err == nil {
		t.Fatal("expected error marking nonexistent notification as read")
	}
}

func TestNotificationService_History_Pagination(t *testing.T) {
	svc, _ := newNotificationService()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := svc.SendPush(ctx, "user-page-1", fmt.Sprintf("Title %d", i), "body", ""); err != nil {
			t.Fatalf("SendPush %d: %v", i, err)
		}
	}

	items, total, err := svc.GetNotificationHistory(ctx, "user-page-1", 1, 20)
	if err != nil {
		t.Fatalf("GetNotificationHistory: %v", err)
	}
	if total != 5 || len(items) != 5 {
		t.Fatalf("expected 5 items, got total=%d len=%d", total, len(items))
	}
}
