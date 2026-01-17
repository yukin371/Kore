package main

// User represents a user in the system
type User struct {
	ID       int
	Name     string
	Email    string
	Password string
}

// NewUser creates a new user
func NewUser(name, email, password string) *User {
	return &User{
		Name:     name,
		Email:    email,
		Password: hashPassword(password),
	}
}

func hashPassword(pwd string) string {
	// Simplified password hashing
	return "hashed_" + pwd
}
