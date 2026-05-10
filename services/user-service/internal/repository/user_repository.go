package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/Temych228/AP2_Final_OnlineParking/services/user-service/internal/domain"
)

const (
	cacheKeyID    = "user:id:"
	cacheKeyEmail = "user:email:"
)

type UserRepository struct {
	db    *pgxpool.Pool
	cache *redis.Client
	ttl   time.Duration
}

func NewUserRepository(db *pgxpool.Pool, cache *redis.Client, ttl time.Duration) *UserRepository {
	return &UserRepository{db: db, cache: cache, ttl: ttl}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.ErrInvalidInput
	}

	if user, err := r.getFromCache(ctx, cacheKeyID+id); err == nil {
		return user, nil
	}

	user, err := r.getByIDFromDB(ctx, id)
	if err != nil {
		return nil, err
	}

	r.setCache(ctx, user)
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, domain.ErrInvalidInput
	}

	if user, err := r.getFromCache(ctx, cacheKeyEmail+email); err == nil {
		return user, nil
	}

	query := `
		SELECT id, email, first_name, last_name, phone, role, is_verified, is_banned, COALESCE(ban_reason, ''), created_at, updated_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`
	row := r.db.QueryRow(ctx, query, email)

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	r.setCache(ctx, user)
	return user, nil
}

func (r *UserRepository) List(ctx context.Context, page, pageSize int, role string) ([]*domain.User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	var (
		countQuery string
		query      string
		args       []any
	)

	if role == "" {
		countQuery = `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
		query = `
			SELECT id, email, first_name, last_name, phone, role, is_verified, is_banned, COALESCE(ban_reason, ''), created_at, updated_at
			FROM users
			WHERE deleted_at IS NULL
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		args = []any{pageSize, offset}
	} else {
		countQuery = `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND role = $1`
		query = `
			SELECT id, email, first_name, last_name, phone, role, is_verified, is_banned, COALESCE(ban_reason, ''), created_at, updated_at
			FROM users
			WHERE deleted_at IS NULL AND role = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []any{role, pageSize, offset}
	}

	var total int
	if err := r.db.QueryRow(ctx, countQuery, argsForCount(role)...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}

	return users, total, rows.Err()
}

func (r *UserRepository) GetBatch(ctx context.Context, ids []string) ([]*domain.User, error) {
	if len(ids) == 0 {
		return []*domain.User{}, nil
	}

	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, strings.TrimSpace(id))
	}

	query := fmt.Sprintf(`
		SELECT id, email, first_name, last_name, phone, role, is_verified, is_banned, COALESCE(ban_reason, ''), created_at, updated_at
		FROM users
		WHERE deleted_at IS NULL AND id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get batch users: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func (r *UserRepository) CheckExists(ctx context.Context, email string) (bool, string, error) {
	email = normalizeEmail(email)
	if email == "" {
		return false, "", domain.ErrInvalidInput
	}

	var id string
	err := r.db.QueryRow(ctx, `SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("check user exists: %w", err)
	}
	return true, id, nil
}

func (r *UserRepository) Create(ctx context.Context, input domain.CreateInput) (*domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`, input.Email).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, domain.ErrEmailTaken
	}

	query := `
		INSERT INTO users (email, first_name, last_name, phone, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, first_name, last_name, phone, role, is_verified, is_banned, COALESCE(ban_reason, ''), created_at, updated_at
	`
	row := tx.QueryRow(ctx, query, input.Email, input.FirstName, input.LastName, input.Phone, input.Role)

	user, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	r.setCache(ctx, user)
	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, id string, input domain.UpdateInput) (*domain.User, error) {
	query := `
		UPDATE users
		SET first_name = $2, last_name = $3, phone = $4, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, email, first_name, last_name, phone, role, is_verified, is_banned, COALESCE(ban_reason, ''), created_at, updated_at
	`
	row := r.db.QueryRow(ctx, query, id, input.FirstName, input.LastName, input.Phone)

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("update user: %w", err)
	}

	r.invalidateCache(ctx, user.ID, user.Email)
	r.setCache(ctx, user)

	return user, nil
}

func (r *UserRepository) VerifyEmail(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET is_verified = true, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("verify email: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}

	r.invalidateCacheByID(ctx, id)
	return nil
}

func (r *UserRepository) Ban(ctx context.Context, id, reason string) error {
	tag, err := r.db.Exec(ctx, `UPDATE users SET is_banned = true, ban_reason = $2, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id, reason)
	if err != nil {
		return fmt.Errorf("ban user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}

	r.invalidateCacheByID(ctx, id)
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var email string
	err = tx.QueryRow(ctx, `UPDATE users SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL RETURNING email`, id).Scan(&email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUserNotFound
		}
		return fmt.Errorf("delete user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	r.invalidateCache(ctx, id, email)
	return nil
}

func (r *UserRepository) getByIDFromDB(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, first_name, last_name, phone, role, is_verified, is_banned, COALESCE(ban_reason, ''), created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	row := r.db.QueryRow(ctx, query, id)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func (r *UserRepository) getFromCache(ctx context.Context, key string) (*domain.User, error) {
	if r.cache == nil {
		return nil, redis.Nil
	}

	data, err := r.cache.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var user domain.User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) setCache(ctx context.Context, user *domain.User) {
	if r.cache == nil || user == nil {
		return
	}

	data, err := json.Marshal(user)
	if err != nil {
		log.Printf("cache marshal error: %v", err)
		return
	}

	if err := r.cache.Set(ctx, cacheKeyID+user.ID, data, r.ttl).Err(); err != nil {
		log.Printf("cache set error: %v", err)
	}

	if err := r.cache.Set(ctx, cacheKeyEmail+strings.ToLower(user.Email), data, r.ttl).Err(); err != nil {
		log.Printf("cache set error: %v", err)
	}
}

func (r *UserRepository) invalidateCache(ctx context.Context, id, email string) {
	if r.cache == nil {
		return
	}

	_, _ = r.cache.Del(ctx, cacheKeyID+id, cacheKeyEmail+strings.ToLower(email)).Result()
}

func (r *UserRepository) invalidateCacheByID(ctx context.Context, id string) {
	if r.cache == nil {
		return
	}

	_, _ = r.cache.Del(ctx, cacheKeyID+id).Result()
}

func scanUser(row interface {
	Scan(dest ...any) error
}) (*domain.User, error) {
	var u domain.User
	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.FirstName,
		&u.LastName,
		&u.Phone,
		&u.Role,
		&u.IsVerified,
		&u.IsBanned,
		&u.BanReason,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func argsForCount(role string) []any {
	if role == "" {
		return nil
	}
	return []any{role}
}

func (r *UserRepository) CreateWithID(ctx context.Context, id string, input domain.CreateInput) (*domain.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO users (id, email, first_name, last_name, phone, role)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET email = EXCLUDED.email,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			phone = EXCLUDED.phone,
			role = EXCLUDED.role,
			updated_at = NOW()
		RETURNING id, email, first_name, last_name, phone, role, is_verified, is_banned, COALESCE(ban_reason, ''), created_at, updated_at
	`

	row := tx.QueryRow(ctx, query, id, input.Email, input.FirstName, input.LastName, input.Phone, input.Role)

	user, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("insert user with id: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	r.setCache(ctx, user)
	return user, nil
}
