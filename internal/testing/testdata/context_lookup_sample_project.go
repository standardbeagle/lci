package testdata

import (
	_ "encoding/json"
	_ "fmt"
	_ "net/http"
	_ "strings"
	_ "time"
)

// ContextLookupSampleProject returns a comprehensive sample project for testing context lookup
func ContextLookupSampleProject() map[string]string {
	return map[string]string{
		"main.go": `
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// @lci:labels[critical,security]
// @lci:category[authentication]
func authenticateUser(username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("username and password required")
	}

	// Security vulnerability: SQL injection risk
	// In production, use prepared statements
	query := fmt.Sprintf("SELECT * FROM users WHERE username='%s'", username)
	fmt.Printf("Executing query: %s", query)

	// Simulate database call
	if !isValidCredentials(username, password) {
		return fmt.Errorf("invalid credentials")
	}

	return nil
}

// @lci:labels[performance-cost]
// @lci:depends[user-service,database]
func getUserProfile(userID int) (*UserProfile, error) {
	startTime := time.Now()
	defer func() {
		fmt.Printf("getUserProfile took %v", time.Since(startTime))
	}()

	// External service call
	resp, err := http.Get(fmt.Sprintf("http://user-service/api/users/%d", userID))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user service returned status %d", resp.StatusCode)
	}

	// Parse response
	var profile UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("failed to decode profile: %w", err)
	}

	return &profile, nil
}

// @lci:labels[bug-propagation]
func calculateDiscount(price float64, customerType string) (float64, error) {
	var discount float64

	switch customerType {
	case "premium":
		discount = price * 0.2 // 20% discount
	case "gold":
		discount = price * 0.3 // 30% discount
	case "platinum":
		discount = price * 0.4 // 40% discount
	default:
		discount = price * 0.1 // 10% discount for regular customers
	}

	if discount > price {
		return 0, fmt.Errorf("discount cannot exceed price")
	}

	return price - discount, nil
}

// HTTP handler - entry point
func handleUserLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if err := authenticateUser(username, password); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	userID := 42 // Hardcoded for simplicity
	profile, err := getUserProfile(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Welcome %s! Your email is %s", profile.Name, profile.Email)
}

// Another entry point
func handleUserProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("id")
	if userIDStr == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	var userID int
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	profile, err := getUserProfile(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// Discount calculation handler
func handleCalculateDiscount(w http.ResponseWriter, r *http.Request) {
	priceStr := r.FormValue("price")
	customerType := r.FormValue("type")

	if priceStr == "" || customerType == "" {
		http.Error(w, "price and type required", http.StatusBadRequest)
		return
	}

	var price float64
	if _, err := fmt.Sscanf(priceStr, "%f", &price); err != nil {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	discountedPrice, err := calculateDiscount(price, customerType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Discounted price: $%.2f", discountedPrice)
}

// Helper functions
func isValidCredentials(username, password string) bool {
	// Simulate credential validation
	validCredentials := map[string]string{
		"admin":     "admin123",
		"user1":     "password1",
		"testuser":  "testpass",
	}

	if pass, exists := validCredentials[username]; exists {
		return pass == password
	}
	return false
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/login", handleUserLogin)
	http.HandleFunc("/profile", handleUserProfile)
	http.HandleFunc("/discount", handleCalculateDiscount)

	fmt.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
`,

		"models/user.go": `
package models

import "time"

// UserProfile represents user information
type UserProfile struct {
	ID        int       'json:"id"'
	Name      string    'json:"name"'
	Email     string    'json:"email"'
	CreatedAt time.Time 'json:"created_at"'
	UpdatedAt time.Time 'json:"updated_at"'
	IsActive  bool      'json:"is_active"'
	Profile  Profile   'json:"profile"'
}

// Profile contains additional user profile information
type Profile struct {
	FirstName string   'json:"first_name"'
	LastName  string   'json:"last_name"'
	Phone     string   'json:"phone"'
	Address   string   'json:"address"'
	City      string   'json:"city"'
	Country   string   'json:"country"'
	Interests []string 'json:"interests"'
}

// User represents the user entity
type User struct {
	ID       int        'json:"id"'
	Username string     'json:"username"'
	Email    string     'json:"email"'
	Password string     'json:"-"'
	Roles    []string   'json:"roles"'
	Profile  UserProfile 'json:"profile"'
}

// IsValid checks if user data is valid
func (u *User) IsValid() error {
	if u.Username == "" {
		return fmt.Errorf("username required")
	}
	if u.Email == "" {
		return fmt.Errorf("email required")
	}
	if !strings.Contains(u.Email, "@") {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// HasRole checks if user has a specific role
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin checks if user is an administrator
func (u *User) IsAdmin() bool {
	return u.HasRole("admin")
}
`,

		"services/auth_service.go": `
package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// AuthService handles authentication operations
type AuthService struct {
	tokenSecret []byte
	sessions    map[string]*Session
}

// Session represents a user session
type Session struct {
	ID        string    'json:"id"'
	UserID    int       'json:"user_id"'
	Token     string    'json:"token"'
	CreatedAt time.Time 'json:"created_at"'
	ExpiresAt time.Time 'json:"expires_at"'
	IsActive  bool      'json:"is_active"'
}

// NewAuthService creates a new authentication service
func NewAuthService(secret string) *AuthService {
	return &AuthService{
		tokenSecret: []byte(secret),
		sessions:    make(map[string]*Session),
	}
}

// GenerateToken generates a secure random token
func (as *AuthService) GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// CreateSession creates a new user session
func (as *AuthService) CreateSession(userID int) (*Session, error) {
	token, err := as.GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	session := &Session{
		ID:        fmt.Sprintf("session_%d_%d", userID, time.Now().Unix()),
		UserID:    userID,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hours
		IsActive:  true,
	}

	as.sessions[session.ID] = session
	return session, nil
}

// ValidateToken validates a session token
func (as *AuthService) ValidateToken(token string) (*Session, error) {
	for _, session := range as.sessions {
		if session.Token == token && session.IsActive && time.Now().Before(session.ExpiresAt) {
			return session, nil
		}
	}
	return nil, fmt.Errorf("invalid or expired token")
}

// InvalidateSession invalidates a user session
func (as *AuthService) InvalidateSession(sessionID string) error {
	if session, exists := as.sessions[sessionID]; exists {
		session.IsActive = false
		return nil
	}
	return fmt.Errorf("session not found")
}

// CleanupExpiredSessions removes expired sessions
func (as *AuthService) CleanupExpiredSessions() {
	for id, session := range as.sessions {
		if !session.IsActive || time.Now().After(session.ExpiresAt) {
			delete(as.sessions, id)
		}
	}
}
`,

		"utils/validators.go": `
package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// EmailValidator validates email addresses
type EmailValidator struct {
	pattern *regexp.Regexp
}

// NewEmailValidator creates a new email validator
func NewEmailValidator() *EmailValidator {
	// RFC 5322 compliant email regex (simplified)
	pattern := regexp.MustCompile('^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$')
	return &EmailValidator{pattern: pattern}
}

// IsValid checks if an email address is valid
func (ev *EmailValidator) IsValid(email string) bool {
	if email == "" {
		return false
	}

	email = strings.TrimSpace(strings.ToLower(email))
	return ev.pattern.MatchString(email)
}

// PasswordValidator validates passwords
type PasswordValidator struct {
	minLength    int
	requireUpper  bool
	requireLower  bool
	requireNumber bool
	requireSymbol bool
}

// NewPasswordValidator creates a new password validator
func NewPasswordValidator() *PasswordValidator {
	return &PasswordValidator{
		minLength:    8,
		requireUpper:  true,
		requireLower:  true,
		requireNumber: true,
		requireSymbol: true,
	}
}

// IsValid checks if a password meets the requirements
func (pv *PasswordValidator) IsValid(password string) error {
	if len(password) < pv.minLength {
		return fmt.Errorf("password must be at least %d characters", pv.minLength)
	}

	if pv.requireUpper {
		hasUpper := false
		for _, c := range password {
			if c >= 'A' && c <= 'Z' {
				hasUpper = true
				break
			}
		}
		if !hasUpper {
			return fmt.Errorf("password must contain at least one uppercase letter")
		}
	}

	if pv.requireLower {
		hasLower := false
		for _, c := range password {
			if c >= 'a' && c <= 'z' {
				hasLower = true
				break
			}
		}
		if !hasLower {
			return fmt.Errorf("password must contain at least one lowercase letter")
		}
	}

	if pv.requireNumber {
		hasNumber := false
		for _, c := range password {
			if c >= '0' && c <= '9' {
				hasNumber = true
				break
			}
		}
		if !hasNumber {
			return fmt.Errorf("password must contain at least one number")
		}
	}

	if pv.requireSymbol {
		hasSymbol := false
		symbols := "!@#$%^&*()_+-=[]{}|;:,.<>?"
		for _, c := range password {
			if strings.Contains(symbols, string(c)) {
				hasSymbol = true
				break
			}
		}
		if !hasSymbol {
			return fmt.Errorf("password must contain at least one special character")
		}
	}

	return nil
}

// ValidateInput performs common input validation
func ValidateInput(input string, inputType string) error {
	switch strings.ToLower(inputType) {
	case "email":
		validator := NewEmailValidator()
		if !validator.IsValid(input) {
			return fmt.Errorf("invalid email address")
		}
	case "username":
		if len(input) < 3 {
			return fmt.Errorf("username must be at least 3 characters")
		}
		if len(input) > 20 {
			return fmt.Errorf("username must not exceed 20 characters")
		}
		if !regexp.MustCompile('^[a-zA-Z0-9_-]+$').MatchString(input) {
			return fmt.Errorf("username can only contain letters, numbers, underscores, and hyphens")
		}
	case "password":
		validator := NewPasswordValidator()
		return validator.IsValid(input)
	default:
		if strings.TrimSpace(input) == "" {
			return fmt.Errorf("input cannot be empty")
		}
	}
	return nil
}
`,

		"config/config.go": `
package config

import (
	"os"
	"strconv"
	"time"
)

// Config represents application configuration
type Config struct {
	Server   ServerConfig   'json:"server"'
	Database DatabaseConfig 'json:"database"'
	Auth     AuthConfig     'json:"auth"'
	Security SecurityConfig 'json:"security"'
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Port         string        'json:"port"'
	Host         string        'json:"host"'
	ReadTimeout  time.Duration 'json:"read_timeout"'
	WriteTimeout time.Duration 'json:"write_timeout"'
	IdleTimeout  time.Duration 'json:"idle_timeout"'
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Host     string 'json:"host"'
	Port     int    'json:"port"'
	Name     string 'json:"name"'
	User     string 'json:"user"'
	Password string 'json:"password"'
	SSL      bool   'json:"ssl"'
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecret     string        'json:"jwt_secret"'
	TokenExpiry   time.Duration 'json:"token_expiry"'
	RefreshExpiry time.Duration 'json:"refresh_expiry"'
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	RateLimitRequests int           'json:"rate_limit_requests"'
	RateLimitWindow   time.Duration 'json:"rate_limit_window"'
	EnableCORS        bool          'json:"enable_cors"'
	AllowedOrigins    []string      'json:"allowed_origins"'
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			Host:         getEnv("SERVER_HOST", "localhost"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getDurationEnv("SERVER_IDLE_TIMEOUT", 120*time.Second),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getIntEnv("DB_PORT", 5432),
			Name:     getEnv("DB_NAME", "app_db"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			SSL:      getBoolEnv("DB_SSL", false),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv("JWT_SECRET", "default-secret-change-in-production"),
			TokenExpiry:   getDurationEnv("TOKEN_EXPIRY", 24*time.Hour),
			RefreshExpiry: getDurationEnv("REFRESH_EXPIRY", 7*24*time.Hour),
		},
		Security: SecurityConfig{
			RateLimitRequests: getIntEnv("RATE_LIMIT_REQUESTS", 100),
			RateLimitWindow:   getDurationEnv("RATE_LIMIT_WINDOW", time.Minute),
			EnableCORS:        getBoolEnv("ENABLE_CORS", true),
			AllowedOrigins:    []string{"http://localhost:3000", "http://localhost:8080"},
		},
	}

	return config, nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
`,

		"README.md": `
# Context Lookup Sample Project

This is a comprehensive sample project designed to test the Code Object Context Lookup tool. It contains various code patterns and relationships that can be analyzed.

## Project Structure

- **main.go**: Main application with HTTP handlers and business logic
- **models/user.go**: User data models with validation methods
- **services/auth_service.go**: Authentication service with session management
- **utils/validators.go**: Input validation utilities
- **config/config.go**: Configuration management

## Key Features for Testing

### 1. Multiple Symbol Types
- Functions (authenticateUser, getUserProfile, calculateDiscount)
- Methods (Process, IsValid, HasRole)
- Classes/Structs (User, UserProfile, AuthService)
- Entry points (HTTP handlers)

### 2. Various Relationships
- Function calls and dependencies
- Class inheritance and composition
- Service dependencies and external API calls
- Global variable usage

### 3. @lci: Annotations
- Security labels: @lci:labels[critical,security]
- Bug propagation: @lci:labels[bug-propagation]
- Performance costs: @lci:labels[performance-cost]
- Dependencies: @lci:depends[user-service,database]
- Categories: @lci:category[authentication]

### 4. Complexity Patterns
- Simple functions vs complex functions
- Functions with different parameter counts
- Nested logic and error handling
- Concurrent code patterns

### 5. Real-world Scenarios
- Authentication flow
- Database operations
- HTTP API endpoints
- Configuration management
- Input validation
- Error handling

## Usage Examples

The sample project can be used to test various context lookup scenarios:

1. **Basic Symbol Lookup**: Find where functions are defined and used
2. **Relationship Analysis**: Understand how components interact
3. **Semantic Context**: Analyze @lci: annotations and propagation
4. **Usage Patterns**: Identify hotspots and critical paths
5. **Change Impact**: Assess the impact of making changes
6. **Code Quality**: Detect code smells and suggest improvements
`,
	}
}