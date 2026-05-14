package domain

import (
	"errors"
	"strings"
	"unicode"
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

func ValidatePassword(password string) error {
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return ErrInvalidInput
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return ErrInvalidInput
	}

	return nil
}

func (i *RegisterInput) Validate() error {
	i.Email = strings.TrimSpace(strings.ToLower(i.Email))
	i.Password = strings.TrimSpace(i.Password)
	i.FirstName = strings.TrimSpace(i.FirstName)
	i.LastName = strings.TrimSpace(i.LastName)
	i.Phone = strings.TrimSpace(i.Phone)

	if i.Email == "" || i.Password == "" || i.FirstName == "" || i.LastName == "" {
		return ErrInvalidInput
	}

	return ValidatePassword(i.Password)
}

func (i *LoginInput) Validate() error {
	i.Email = strings.TrimSpace(strings.ToLower(i.Email))
	i.Password = strings.TrimSpace(i.Password)

	if i.Email == "" || i.Password == "" {
		return ErrInvalidInput
	}

	return nil
}
