package main

import (
	"fmt"
	"log"
	"net/http"
)

// UserService handles user-related operations
type UserService struct {
	db Database
}

// Database interface for data persistence
type Database interface {
	FindUser(id string) (*User, error)
	SaveUser(user *User) error
}

// User represents a user in the system
type User struct {
	ID       string
	Username string
	Email    string
}

// NewUserService creates a new UserService instance
func NewUserService(db Database) *UserService {
	return &UserService{db: db}
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id string) (*User, error) {
	return s.db.FindUser(id)
}

// CreateUser creates a new user
func (s *UserService) CreateUser(username, email string) error {
	user := &User{
		Username: username,
		Email:    email,
	}
	return s.db.SaveUser(user)
}

// HandleUserRequest processes HTTP requests for user operations
func HandleUserRequest(w http.ResponseWriter, r *http.Request) {
	// Handle user request logic
	fmt.Fprintf(w, "User request handled")
}

func main() {
	http.HandleFunc("/users", HandleUserRequest)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
