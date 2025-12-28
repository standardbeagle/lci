package main

import (
	"errors"
	"time"
)

// AuthService handles authentication
type AuthService struct {
	userService *UserService
}

// Token represents an authentication token
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// NewAuthService creates a new AuthService
func NewAuthService(userService *UserService) *AuthService {
	return &AuthService{userService: userService}
}

// Authenticate authenticates a user
func (a *AuthService) Authenticate(username, password string) (*Token, error) {
	if username == "" || password == "" {
		return nil, errors.New("invalid credentials")
	}

	token := &Token{
		Value:     "token-value",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	return token, nil
}

// ValidateToken validates an authentication token
func (a *AuthService) ValidateToken(token string) error {
	if token == "" {
		return errors.New("invalid token")
	}
	return nil
}
