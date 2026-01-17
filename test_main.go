package main

import "fmt"

func main() {
	// Initialize auth service
	auth := NewAuthService()

	// Register users
	user1 := auth.Register("Alice", "alice@example.com", "password123")
	user2 := auth.Register("Bob", "bob@example.com", "secret456")

	fmt.Printf("Registered users: %s and %s\n", user1.Name, user2.Name)

	// Test login
	loggedInUser, err := auth.Login("alice@example.com", "password123")
	if err != nil {
		fmt.Println("Login failed:", err)
	} else {
		fmt.Printf("Login successful: %s (%s)\n", loggedInUser.Name, loggedInUser.Email)
	}

	// Test get user by ID
	user := auth.GetUserByID(user1.ID)
	if user != nil {
		fmt.Printf("Found user: %s\n", user.Name)
	}
}
