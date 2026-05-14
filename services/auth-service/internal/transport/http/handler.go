package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/service"
)

type Handler struct {
	svc *service.AuthService
}

func New(svc *service.AuthService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/ready", h.health)
	mux.HandleFunc("/register", h.register)
	mux.HandleFunc("/login", h.login)
	mux.HandleFunc("/refresh", h.refresh)
	mux.HandleFunc("/logout", h.logout)
	mux.HandleFunc("/verify-email", h.verifyEmail)
	mux.HandleFunc("/forgot-password", h.forgotPassword)
	mux.HandleFunc("/reset-password", h.resetPassword)
	mux.HandleFunc("/change-password", h.changePassword)
	mux.HandleFunc("/session", h.session)
	mux.HandleFunc("/revoke-all-sessions", h.revokeAllSessions)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID, err := h.svc.Register(r.Context(), domain.RegisterInput{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	})
	if err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		UserID:  userID,
		Message: "verification email sent",
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tokens, err := h.svc.Login(r.Context(), domain.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tokenPairResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
		UserID:       tokens.UserID,
		Email:        tokens.Email,
	})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tokens, err := h.svc.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tokenPairResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
		UserID:       tokens.UserID,
		Email:        tokens.Email,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req logoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req verifyEmailRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.VerifyEmail(r.Context(), req.Token); err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "email verified",
	})
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req forgotPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := h.svc.ForgotPassword(r.Context(), req.Email)
	if err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, forgotPasswordResponse{
		Success: true,
		Message: "password reset token created",
		Token:   token,
	})
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.ChangePassword(r.Context(), req.UserID, req.OldPassword, req.NewPassword); err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("access_token"))
	}
	if token == "" {
		writeError(w, http.StatusBadRequest, "access token is required")
		return
	}

	userID, email, expiresAt, err := h.svc.GetSession(r.Context(), token)
	if err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionResponse{
		UserID:    userID,
		Email:     email,
		Role:      "user",
		ExpiresAt: expiresAt,
	})
}

func (h *Handler) revokeAllSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req revokeAllSessionsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	count, err := h.svc.RevokeAllSessions(r.Context(), req.UserID)
	if err != nil {
		writeMappedError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, revokeAllSessionsResponse{
		RevokedCount: count,
	})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if dec.More() {
		return errors.New("invalid request body")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeMappedError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrTokenExpired):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrTokenNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	parts := strings.Fields(header)
	if len(parts) != 2 {
		return ""
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

type registerResponse struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type forgotPasswordResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type changePasswordRequest struct {
	UserID      string `json:"user_id"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type revokeAllSessionsRequest struct {
	UserID string `json:"user_id"`
}

type revokeAllSessionsResponse struct {
	RevokedCount int32 `json:"revoked_count"`
}

type tokenPairResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
}

type sessionResponse struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}
