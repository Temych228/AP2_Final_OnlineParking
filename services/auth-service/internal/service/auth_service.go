package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/config"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/domain"
	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/repository"
)

type AuthService struct {
	cfg  *config.Config
	repo *repository.AuthRepository
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	UserID       string
	Email        string
}

func New(cfg *config.Config, repo *repository.AuthRepository) *AuthService {
	return &AuthService{cfg: cfg, repo: repo}
}

func (s *AuthService) Register(ctx context.Context, input domain.RegisterInput) (string, error) {
	if err := input.Validate(); err != nil {
		return "", err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	userID, err := s.repo.CreateUser(ctx, input.Email, string(passwordHash))
	if err != nil {
		return "", err
	}

	token := s.newRandomToken()
	expiresAt := time.Now().Add(s.cfg.VerificationTTL)

	if err := s.repo.StoreVerificationToken(ctx, userID, token, expiresAt); err != nil {
		return "", err
	}

	/*
		_ = s.publisher.PublishUserRegistered(userID, input.Email, input.FirstName, verificationToken)
	*/

	return userID, nil
}

func (s *AuthService) Login(ctx context.Context, input domain.LoginInput) (*TokenPair, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	accessToken, expiresAt, err := s.makeAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	refreshToken := s.newRandomToken()
	refreshExpiresAt := time.Now().Add(s.cfg.RefreshTokenTTL)

	if err := s.repo.CreateRefreshToken(ctx, user.ID, refreshToken, refreshExpiresAt); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		UserID:       user.ID,
		Email:        user.Email,
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	userID, expiresAt, ok, err := s.repo.FindRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	if time.Now().After(expiresAt) {
		return nil, domain.ErrTokenExpired
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	_ = s.repo.RevokeRefreshToken(ctx, refreshToken)

	newRefreshToken := s.newRandomToken()
	newRefreshExpiresAt := time.Now().Add(s.cfg.RefreshTokenTTL)
	if err := s.repo.CreateRefreshToken(ctx, user.ID, newRefreshToken, newRefreshExpiresAt); err != nil {
		return nil, err
	}

	accessToken, accessExpiresAt, err := s.makeAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    accessExpiresAt,
		UserID:       user.ID,
		Email:        user.Email,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.RevokeRefreshToken(ctx, refreshToken)
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	userID, err := s.repo.GetVerificationToken(ctx, token)
	if err != nil {
		return err
	}

	if err := s.repo.MarkVerified(ctx, userID); err != nil {
		return err
	}

	return s.repo.DeleteVerificationToken(ctx, token)
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	token := s.newRandomToken()
	expiresAt := time.Now().Add(s.cfg.PasswordResetTTL)

	if err := s.repo.StorePasswordResetToken(ctx, user.ID, token, expiresAt); err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return domain.ErrInvalidInput
	}

	userID, err := s.repo.GetPasswordResetToken(ctx, token)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.repo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return err
	}

	return s.repo.DeletePasswordResetToken(ctx, token)
}

func (s *AuthService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return domain.ErrInvalidInput
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return domain.ErrUnauthorized
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.repo.UpdatePassword(ctx, userID, string(hash))
}

func (s *AuthService) ValidateToken(tokenString string) (string, string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return "", "", domain.ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", domain.ErrUnauthorized
	}

	userID, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	return userID, email, nil
}

func (s *AuthService) GetSession(ctx context.Context, accessToken string) (string, string, time.Time, error) {
	userID, email, err := s.ValidateToken(accessToken)
	if err != nil {
		return "", "", time.Time{}, err
	}

	return userID, email, time.Now().Add(s.cfg.AccessTokenTTL), nil
}

func (s *AuthService) RevokeAllSessions(ctx context.Context, userID string) (int32, error) {
	return s.repo.RevokeAllRefreshTokens(ctx, userID)
}

func (s *AuthService) makeAccessToken(userID, email string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.cfg.AccessTokenTTL)

	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"exp":   expiresAt.Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

func (s *AuthService) newRandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	sum := sha256.Sum256(b)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
