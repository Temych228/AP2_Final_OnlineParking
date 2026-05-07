package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/Temych228/AP2_Final_OnlineParking/services/auth-service/internal/domain"
)

type UserRecord struct {
	ID           string
	Email        string
	PasswordHash string
	IsVerified   bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AuthRepository struct {
	db    *pgxpool.Pool
	cache *redis.Client
}

func New(db *pgxpool.Pool, cache *redis.Client) *AuthRepository {
	return &AuthRepository{db: db, cache: cache}
}

func (r *AuthRepository) DB() *pgxpool.Pool {
	return r.db
}

func (r *AuthRepository) CreateUser(ctx context.Context, email, passwordHash string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO auth_users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id
	`, normalizeEmail(email), passwordHash).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return "", domain.ErrEmailTaken
		}
		return "", fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (*UserRecord, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, is_verified, created_at, updated_at
		FROM auth_users
		WHERE email = $1
	`, normalizeEmail(email))

	var u UserRecord
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return &u, nil
}

func (r *AuthRepository) GetUserByID(ctx context.Context, id string) (*UserRecord, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, is_verified, created_at, updated_at
		FROM auth_users
		WHERE id = $1
	`, strings.TrimSpace(id))

	var u UserRecord
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsVerified, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return &u, nil
}

func (r *AuthRepository) MarkVerified(ctx context.Context, userID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE auth_users
		SET is_verified = true, updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("mark verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *AuthRepository) UpdatePassword(ctx context.Context, userID, newHash string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE auth_users
		SET password_hash = $2, updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(userID), newHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *AuthRepository) CreateRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	hash := hashToken(token)
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, strings.TrimSpace(userID), hash, expiresAt)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *AuthRepository) FindRefreshToken(ctx context.Context, token string) (string, time.Time, bool, error) {
	hash := hashToken(token)

	row := r.db.QueryRow(ctx, `
		SELECT user_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, hash)

	var (
		userID    string
		expiresAt time.Time
		revokedAt *time.Time
	)

	err := row.Scan(&userID, &expiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, false, domain.ErrTokenNotFound
		}
		return "", time.Time{}, false, fmt.Errorf("find refresh token: %w", err)
	}

	if revokedAt != nil {
		return "", time.Time{}, false, domain.ErrUnauthorized
	}

	return userID, expiresAt, true, nil
}

func (r *AuthRepository) RevokeRefreshToken(ctx context.Context, token string) error {
	hash := hashToken(token)
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, hash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *AuthRepository) RevokeAllRefreshTokens(ctx context.Context, userID string) (int32, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, strings.TrimSpace(userID))
	if err != nil {
		return 0, fmt.Errorf("revoke all refresh tokens: %w", err)
	}
	return int32(tag.RowsAffected()), nil
}

func (r *AuthRepository) StoreVerificationToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	return r.storeTempToken(ctx, "verify:"+token, userID, expiresAt)
}

func (r *AuthRepository) GetVerificationToken(ctx context.Context, token string) (string, error) {
	return r.getTempToken(ctx, "verify:"+token)
}

func (r *AuthRepository) DeleteVerificationToken(ctx context.Context, token string) error {
	return r.deleteTempToken(ctx, "verify:"+token)
}

func (r *AuthRepository) StorePasswordResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	return r.storeTempToken(ctx, "reset:"+token, userID, expiresAt)
}

func (r *AuthRepository) GetPasswordResetToken(ctx context.Context, token string) (string, error) {
	return r.getTempToken(ctx, "reset:"+token)
}

func (r *AuthRepository) DeletePasswordResetToken(ctx context.Context, token string) error {
	return r.deleteTempToken(ctx, "reset:"+token)
}

func (r *AuthRepository) storeTempToken(ctx context.Context, key, value string, expiresAt time.Time) error {
	if r.cache == nil {
		return nil
	}
	return r.cache.Set(ctx, key, value, time.Until(expiresAt)).Err()
}

func (r *AuthRepository) getTempToken(ctx context.Context, key string) (string, error) {
	if r.cache == nil {
		return "", domain.ErrTokenNotFound
	}
	val, err := r.cache.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", domain.ErrTokenNotFound
		}
		return "", fmt.Errorf("get temp token: %w", err)
	}
	return val, nil
}

func (r *AuthRepository) deleteTempToken(ctx context.Context, key string) error {
	if r.cache == nil {
		return nil
	}
	_, err := r.cache.Del(ctx, key).Result()
	return err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
