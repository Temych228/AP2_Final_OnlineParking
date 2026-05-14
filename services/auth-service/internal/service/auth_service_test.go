package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/publisher"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/repository"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/service"
	httptransport "github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/transport/http"
	_ "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authRefreshRow struct {
	userID    string
	expiresAt time.Time
	revoked   bool
}

type fakeAuthRepo struct {
	usersByEmail map[string]*repository.UserRecord
	usersByID    map[string]*repository.UserRecord
	refresh      map[string]authRefreshRow
	verifyTokens map[string]string
	resetTokens  map[string]string
	nextID       int
}

func newFakeAuthRepo() *fakeAuthRepo {
	return &fakeAuthRepo{
		usersByEmail: make(map[string]*repository.UserRecord),
		usersByID:    make(map[string]*repository.UserRecord),
		refresh:      make(map[string]authRefreshRow),
		verifyTokens: make(map[string]string),
		resetTokens:  make(map[string]string),
		nextID:       1,
	}
}

func (r *fakeAuthRepo) CreateUser(_ context.Context, email, passwordHash string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, exists := r.usersByEmail[email]; exists {
		return "", domain.ErrEmailTaken
	}

	id := fmt.Sprintf("user-%d", r.nextID)
	r.nextID++

	now := time.Now().UTC()
	user := &repository.UserRecord{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		IsVerified:   false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r.usersByEmail[email] = user
	r.usersByID[id] = user
	return id, nil
}

func (r *fakeAuthRepo) GetUserByEmail(_ context.Context, email string) (*repository.UserRecord, error) {
	if u, ok := r.usersByEmail[strings.ToLower(strings.TrimSpace(email))]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}

func (r *fakeAuthRepo) GetUserByID(_ context.Context, id string) (*repository.UserRecord, error) {
	if u, ok := r.usersByID[strings.TrimSpace(id)]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}

func (r *fakeAuthRepo) MarkVerified(_ context.Context, userID string) error {
	u, ok := r.usersByID[strings.TrimSpace(userID)]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.IsVerified = true
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *fakeAuthRepo) UpdatePassword(_ context.Context, userID, newHash string) error {
	u, ok := r.usersByID[strings.TrimSpace(userID)]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.PasswordHash = newHash
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *fakeAuthRepo) CreateRefreshToken(_ context.Context, userID, token string, expiresAt time.Time) error {
	r.refresh[token] = authRefreshRow{userID: strings.TrimSpace(userID), expiresAt: expiresAt}
	return nil
}

func (r *fakeAuthRepo) FindRefreshToken(_ context.Context, token string) (string, time.Time, bool, error) {
	row, ok := r.refresh[token]
	if !ok {
		return "", time.Time{}, false, domain.ErrTokenNotFound
	}
	if row.revoked {
		return "", time.Time{}, false, domain.ErrUnauthorized
	}
	return row.userID, row.expiresAt, true, nil
}

func (r *fakeAuthRepo) RevokeRefreshToken(_ context.Context, token string) error {
	row, ok := r.refresh[token]
	if !ok {
		return nil
	}
	row.revoked = true
	r.refresh[token] = row
	return nil
}

func (r *fakeAuthRepo) RevokeAllRefreshTokens(_ context.Context, userID string) (int32, error) {
	var count int32
	for token, row := range r.refresh {
		if row.userID == strings.TrimSpace(userID) && !row.revoked {
			row.revoked = true
			r.refresh[token] = row
			count++
		}
	}
	return count, nil
}

func (r *fakeAuthRepo) StoreVerificationToken(_ context.Context, userID, token string, _ time.Time) error {
	r.verifyTokens[token] = strings.TrimSpace(userID)
	return nil
}

func (r *fakeAuthRepo) GetVerificationToken(_ context.Context, token string) (string, error) {
	userID, ok := r.verifyTokens[token]
	if !ok {
		return "", domain.ErrTokenNotFound
	}
	return userID, nil
}

func (r *fakeAuthRepo) DeleteVerificationToken(_ context.Context, token string) error {
	delete(r.verifyTokens, token)
	return nil
}

func (r *fakeAuthRepo) StorePasswordResetToken(_ context.Context, userID, token string, _ time.Time) error {
	r.resetTokens[token] = strings.TrimSpace(userID)
	return nil
}

func (r *fakeAuthRepo) GetPasswordResetToken(_ context.Context, token string) (string, error) {
	userID, ok := r.resetTokens[token]
	if !ok {
		return "", domain.ErrTokenNotFound
	}
	return userID, nil
}

func (r *fakeAuthRepo) DeletePasswordResetToken(_ context.Context, token string) error {
	delete(r.resetTokens, token)
	return nil
}

func newAuthService() (*service.AuthService, *fakeAuthRepo) {
	repo := newFakeAuthRepo()
	cfg := &config.Config{
		JWTSecret:        "test-secret",
		AccessTokenTTL:   time.Hour,
		RefreshTokenTTL:  time.Hour,
		VerificationTTL:  time.Hour,
		PasswordResetTTL: time.Hour,
	}
	return service.New(cfg, repo, (*publisher.NATSPublisher)(nil)), repo
}

func TestAuthService_Unit(t *testing.T) {
	svc, repo := newAuthService()
	ctx := context.Background()

	userID, err := svc.Register(ctx, domain.RegisterInput{
		Email:     "User@Mail.Com",
		Password:  "Password123!",
		FirstName: "Test",
		LastName:  "User",
		Phone:     "+77001112233",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if userID == "" {
		t.Fatal("expected user id")
	}

	if _, err := svc.Register(ctx, domain.RegisterInput{
		Email:     "User@Mail.Com",
		Password:  "Password123!",
		FirstName: "Test",
		LastName:  "User",
		Phone:     "+77001112233",
	}); err == nil {
		t.Fatal("expected duplicate email error")
	}

	tokens, err := svc.Login(ctx, domain.LoginInput{
		Email:    "user@mail.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}

	userIDFromJWT, emailFromJWT, err := svc.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if userIDFromJWT != userID || emailFromJWT != "user@mail.com" {
		t.Fatalf("unexpected jwt claims: %s %s", userIDFromJWT, emailFromJWT)
	}

	sessionUserID, sessionEmail, exp, err := svc.GetSession(ctx, tokens.AccessToken)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sessionUserID != userID || sessionEmail != "user@mail.com" || exp.IsZero() {
		t.Fatalf("unexpected session: %s %s %v", sessionUserID, sessionEmail, exp)
	}

	refreshed, err := svc.RefreshToken(ctx, tokens.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.RefreshToken == tokens.RefreshToken {
		t.Fatal("expected new refresh token")
	}

	verifyToken, err := svc.ForgotPassword(ctx, "user@mail.com")
	if err != nil {
		t.Fatalf("forgot password: %v", err)
	}
	if verifyToken == "" {
		t.Fatal("expected reset token")
	}

	if err := svc.ResetPassword(ctx, verifyToken, "NewPassword123!"); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	if err := svc.ChangePassword(ctx, userID, "NewPassword123!", "AnotherPass123!"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	verTok := "verify-me"
	if err := repo.StoreVerificationToken(ctx, userID, verTok, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("store verify token: %v", err)
	}
	if err := svc.VerifyEmail(ctx, verTok); err != nil {
		t.Fatalf("verify email: %v", err)
	}
	if u := repo.usersByID[userID]; u == nil || !u.IsVerified {
		t.Fatal("expected verified user")
	}

	count, err := svc.RevokeAllSessions(ctx, userID)
	if err != nil {
		t.Fatalf("revoke all sessions: %v", err)
	}
	if count == 0 {
		t.Fatal("expected revoked sessions > 0")
	}
}

func TestAuthService_Errors(t *testing.T) {
	svc, repo := newAuthService()
	ctx := context.Background()

	if _, err := svc.Register(ctx, domain.RegisterInput{Email: "bad", Password: "short"}); err == nil {
		t.Fatal("expected invalid input error")
	}

	if _, err := svc.Login(ctx, domain.LoginInput{Email: "missing@mail.com", Password: "Password123!"}); err == nil {
		t.Fatal("expected missing user error")
	}

	repo.refresh["expired-token"] = authRefreshRow{
		userID:    "user-1",
		expiresAt: time.Now().Add(-time.Hour),
	}
	repo.usersByID["user-1"] = &repository.UserRecord{
		ID:           "user-1",
		Email:        "user@mail.com",
		PasswordHash: mustHash(t, "Password123!"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if _, err := svc.RefreshToken(ctx, "expired-token"); err == nil || !strings.Contains(err.Error(), domain.ErrTokenExpired.Error()) {
		t.Fatalf("expected expired token error, got %v", err)
	}

	if err := svc.ResetPassword(ctx, "missing-token", "NewPassword123!"); err == nil {
		t.Fatal("expected reset token error")
	}

	if err := svc.ChangePassword(ctx, "missing-user", "old", "NewPassword123!"); err == nil {
		t.Fatal("expected change password error")
	}
}

func TestAuthHTTP_Integration(t *testing.T) {
	svc, _ := newAuthService()
	handler := httptransport.New(svc)
	mux := http.NewServeMux()
	handler.Register(mux)

	registerResp := httptest.NewRecorder()
	mux.ServeHTTP(registerResp, httptest.NewRequest(http.MethodPost, "/register", mustJSON(t, map[string]any{
		"email":      "frontend@mail.com",
		"password":   "Password123!",
		"first_name": "Front",
		"last_name":  "End",
		"phone":      "+77001112233",
	})))
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d body=%s", registerResp.Code, registerResp.Body.String())
	}

	loginResp := httptest.NewRecorder()
	mux.ServeHTTP(loginResp, httptest.NewRequest(http.MethodPost, "/login", mustJSON(t, map[string]any{
		"email":    "frontend@mail.com",
		"password": "Password123!",
	})))
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", loginResp.Code, loginResp.Body.String())
	}

	var loginBody map[string]any
	if err := json.Unmarshal(loginResp.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	accessToken, _ := loginBody["access_token"].(string)
	refreshToken, _ := loginBody["refresh_token"].(string)
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("unexpected login response: %s", loginResp.Body.String())
	}

	sessionResp := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodGet, "/session", nil)
	sessionReq.Header.Set("Authorization", "Bearer "+accessToken)
	mux.ServeHTTP(sessionResp, sessionReq)
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("session status = %d body=%s", sessionResp.Code, sessionResp.Body.String())
	}

	refreshResp := httptest.NewRecorder()
	mux.ServeHTTP(refreshResp, httptest.NewRequest(http.MethodPost, "/refresh", mustJSON(t, map[string]any{
		"refresh_token": refreshToken,
	})))
	if refreshResp.Code != http.StatusOK {
		t.Fatalf("refresh status = %d body=%s", refreshResp.Code, refreshResp.Body.String())
	}

	forgotResp := httptest.NewRecorder()
	mux.ServeHTTP(forgotResp, httptest.NewRequest(http.MethodPost, "/forgot-password", mustJSON(t, map[string]any{
		"email": "frontend@mail.com",
	})))
	if forgotResp.Code != http.StatusOK {
		t.Fatalf("forgot password status = %d body=%s", forgotResp.Code, forgotResp.Body.String())
	}

	var forgotBody map[string]any
	if err := json.Unmarshal(forgotResp.Body.Bytes(), &forgotBody); err != nil {
		t.Fatalf("decode forgot response: %v", err)
	}
	resetToken, _ := forgotBody["token"].(string)
	if resetToken == "" {
		t.Fatalf("expected reset token, got %s", forgotResp.Body.String())
	}

	resetResp := httptest.NewRecorder()
	mux.ServeHTTP(resetResp, httptest.NewRequest(http.MethodPost, "/reset-password", mustJSON(t, map[string]any{
		"token":        resetToken,
		"new_password": "NewPassword123!",
	})))
	if resetResp.Code != http.StatusOK {
		t.Fatalf("reset password status = %d body=%s", resetResp.Code, resetResp.Body.String())
	}

	changeResp := httptest.NewRecorder()
	mux.ServeHTTP(changeResp, httptest.NewRequest(http.MethodPost, "/change-password", mustJSON(t, map[string]any{
		"user_id":      "user-1",
		"old_password": "NewPassword123!",
		"new_password": "AnotherPass123!",
	})))
	if changeResp.Code != http.StatusOK {
		t.Fatalf("change password status = %d body=%s", changeResp.Code, changeResp.Body.String())
	}

	revokeResp := httptest.NewRecorder()
	mux.ServeHTTP(revokeResp, httptest.NewRequest(http.MethodPost, "/revoke-all-sessions", mustJSON(t, map[string]any{
		"user_id": "user-1",
	})))
	if revokeResp.Code != http.StatusOK {
		t.Fatalf("revoke all status = %d body=%s", revokeResp.Code, revokeResp.Body.String())
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

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(hash)
}
