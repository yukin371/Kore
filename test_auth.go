package main

import "fmt"

// AuthService handles authentication
type AuthService struct {
	users []*User
}

// NewAuthService creates a new authentication service
func NewAuthService() *AuthService {
	return &AuthService{
		users: make([]*User, 0),
	}
}

// Register registers a new user
func (a *AuthService) Register(name, email, password string) *User {
	user := NewUser(name, email, password)
	a.users = append(a.users, user)
	return user
}

// Login authenticates a user
func (a *AuthService) Login(email, password string) (*User, error) {
	for _, user := range a.users {
		if user.Email == email && user.Password == hashPassword(password) {
			return user, nil
		}
	}
	return nil, fmt.Errorf("invalid credentials")
}

// GetUserByID retrieves a user by ID
func (a *AuthService) GetUserByID(id int) *User {
	for _, user := range a.users {
		if user.ID == id {
			return user
		}
	}
	return nil
}
