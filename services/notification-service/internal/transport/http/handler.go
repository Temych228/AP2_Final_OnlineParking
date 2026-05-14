package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/notification-service/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	svc *service.NotificationService
}

type sendEmailRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	UserID  string `json:"user_id"`
	Type    string `json:"type"`
}

type bulkEmailRequest struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	Type    string   `json:"type"`
}

type sendSMSRequest struct {
	Phone   string `json:"phone"`
	Message string `json:"message"`
	UserID  string `json:"user_id"`
}

type sendPushRequest struct {
	UserID string `json:"user_id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Data   string `json:"data"`
}

type updatePreferencesRequest struct {
	UserID         string `json:"user_id"`
	EmailEnabled   bool   `json:"email_enabled"`
	SMSEnabled     bool   `json:"sms_enabled"`
	PushEnabled    bool   `json:"push_enabled"`
	MarketingEmail bool   `json:"marketing_emails"`
}

type markReadRequest struct {
	UserID string `json:"user_id"`
}

func New(svc *service.NotificationService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/ready", h.health)
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/notifications/health", h.health)
	mux.HandleFunc("/notifications/ready", h.health)

	mux.HandleFunc("/notifications/email", h.sendEmail)
	mux.HandleFunc("/notifications/bulk-email", h.sendBulkEmail)
	mux.HandleFunc("/notifications/history", h.history)
	mux.HandleFunc("/notifications/unread-count", h.unreadCount)
	mux.HandleFunc("/notifications/preferences", h.preferences)
	mux.HandleFunc("/notifications/sms", h.sendSMS)
	mux.HandleFunc("/notifications/push", h.sendPush)

	mux.HandleFunc("/notifications/templates/", h.getTemplate)
	mux.HandleFunc("/notifications/", h.notificationsByID)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "notification-service",
	})
}

func (h *Handler) notificationsByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/notifications/")
	if path == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch {
	case len(parts) == 2 && parts[1] == "read" && r.Method == http.MethodPost:
		h.markRead(w, r, parts[0])
		return
	case len(parts) == 1 && r.Method == http.MethodDelete:
		h.deleteNotification(w, r, parts[0])
		return
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) sendEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req sendEmailRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	n, err := h.svc.SendEmail(r.Context(), req.To, req.Subject, req.Body, req.UserID, req.Type)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success":      true,
		"notification": n,
	})
}

func (h *Handler) sendBulkEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req bulkEmailRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sent, failed, err := h.svc.SendBulkEmail(r.Context(), req.To, req.Subject, req.Body, req.Type)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"sent":    sent,
		"failed":  failed,
	})
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	page := parseInt(r.URL.Query().Get("page"), 1)
	pageSize := parseInt(r.URL.Query().Get("page_size"), 20)

	items, total, err := h.svc.GetNotificationHistory(r.Context(), userID, page, pageSize)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *Handler) unreadCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	count, err := h.svc.GetUnreadCount(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": userID,
		"count":   count,
	})
}

func (h *Handler) preferences(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" {
			writeError(w, http.StatusBadRequest, "user_id is required")
			return
		}

		p, err := h.svc.GetPreferences(r.Context(), userID)
		if err != nil {
			writeDomainError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"preferences": p,
		})

	case http.MethodPut:
		var req updatePreferencesRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if strings.TrimSpace(req.UserID) == "" {
			writeError(w, http.StatusBadRequest, "user_id is required")
			return
		}

		err := h.svc.UpdatePreferences(r.Context(), &domain.Preferences{
			UserID:         req.UserID,
			EmailEnabled:   req.EmailEnabled,
			SMSEnabled:     req.SMSEnabled,
			PushEnabled:    req.PushEnabled,
			MarketingEmail: req.MarketingEmail,
		})
		if err != nil {
			writeDomainError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) sendSMS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req sendSMSRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	n, err := h.svc.SendSMS(r.Context(), req.Phone, req.Message, req.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success":      true,
		"notification": n,
	})
}

func (h *Handler) sendPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req sendPushRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	n, err := h.svc.SendPush(r.Context(), req.UserID, req.Title, req.Body, req.Data)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success":      true,
		"notification": n,
	})
}

func (h *Handler) getTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/notifications/templates/")
	name = strings.TrimSpace(name)

	subject, body := h.svc.GetTemplate(name)
	if subject == "" && body == "" {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"template_name": name,
		"subject":       subject,
		"body":          body,
	})
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request, notificationID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req markReadRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(req.UserID) == "" {
		req.UserID = strings.TrimSpace(r.URL.Query().Get("user_id"))
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if err := h.svc.MarkNotificationRead(r.Context(), notificationID, req.UserID); err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (h *Handler) deleteNotification(w http.ResponseWriter, r *http.Request, notificationID string) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if err := h.svc.DeleteNotification(r.Context(), notificationID, userID); err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]any{
		"success": false,
		"error":   message,
	})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		msg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(msg, "not found"):
			writeError(w, http.StatusNotFound, err.Error())
		case strings.Contains(msg, "invalid"):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
	}
}

func parseInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	v, err := strconv.Atoi(value)
	if err != nil || v < 1 {
		return fallback
	}
	return v
}
