package domain

import (
	"errors"
	"strings"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrEmailTaken    = errors.New("email already taken")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrTokenExpired  = errors.New("token expired")
	ErrTokenInvalid  = errors.New("token invalid")
	ErrTokenNotFound = errors.New("token not found")
)

type RegisterInput struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
	Phone     string
}

type LoginInput struct {
	Email    string
	Password string
}

func (i *RegisterInput) Validate() error {
	i.Email = strings.TrimSpace(strings.ToLower(i.Email))
	i.Password = strings.TrimSpace(i.Password)
	i.FirstName = strings.TrimSpace(i.FirstName)
	i.LastName = strings.TrimSpace(i.LastName)
	i.Phone = strings.TrimSpace(i.Phone)

	if i.Email == "" || i.Password == "" || i.FirstName == "" || i.LastName == "" || i.Phone == "" {
		return ErrInvalidInput
	}

	if len(i.Password) < 8 {
		return ErrInvalidInput
	}

	return nil
}

func (i *LoginInput) Validate() error {
	i.Email = strings.TrimSpace(strings.ToLower(i.Email))
	i.Password = strings.TrimSpace(i.Password)

	if i.Email == "" || i.Password == "" {
		return ErrInvalidInput
	}

	return nil
}
