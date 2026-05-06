package domain

import (
	"errors"
	"strings"
	"time"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID         string    `db:"id"`
	Email      string    `db:"email"`
	FirstName  string    `db:"first_name"`
	LastName   string    `db:"last_name"`
	Phone      string    `db:"phone"`
	Role       Role      `db:"role"`
	IsVerified bool      `db:"is_verified"`
	IsBanned   bool      `db:"is_banned"`
	BanReason  string    `db:"ban_reason"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

type CreateInput struct {
	Email     string
	FirstName string
	LastName  string
	Phone     string
	Role      Role
}

type UpdateInput struct {
	FirstName string
	LastName  string
	Phone     string
}

var (
	ErrUserNotFound = errors.New("user not found")
	ErrEmailTaken   = errors.New("email already taken")
	ErrUserBanned   = errors.New("user is banned")
	ErrInvalidInput = errors.New("invalid input")
)

func (i *CreateInput) Validate() error {
	i.Email = strings.TrimSpace(strings.ToLower(i.Email))
	i.FirstName = strings.TrimSpace(i.FirstName)
	i.LastName = strings.TrimSpace(i.LastName)
	i.Phone = strings.TrimSpace(i.Phone)

	if i.Email == "" || i.FirstName == "" || i.LastName == "" || i.Phone == "" {
		return ErrInvalidInput
	}

	if i.Role == "" {
		i.Role = RoleUser
	}

	if i.Role != RoleUser && i.Role != RoleAdmin {
		return ErrInvalidInput
	}

	return nil
}

func (i *UpdateInput) Validate() error {
	i.FirstName = strings.TrimSpace(i.FirstName)
	i.LastName = strings.TrimSpace(i.LastName)
	i.Phone = strings.TrimSpace(i.Phone)

	if i.FirstName == "" || i.LastName == "" || i.Phone == "" {
		return ErrInvalidInput
	}

	return nil
}
