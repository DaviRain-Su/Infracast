// Package user provides user management APIs
package user

import (
	"context"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
)

// User represents a user in the system
type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// RegisterParams contains registration parameters
type RegisterParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// RegisterResponse contains registration response
type RegisterResponse struct {
	UserID int `json:"user_id"`
}

// Register creates a new user account
//encore:api public method=POST path=/user.register
func Register(ctx context.Context, params *RegisterParams) (*RegisterResponse, error) {
	// Validate input
	if params.Email == "" || params.Password == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "email and password are required",
		}
	}

	// Insert user into database
	var userID int
	err := usersDB.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id
	`, params.Email, hashPassword(params.Password), params.Name).Scan(&userID)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.AlreadyExists,
			Message: "user already exists",
		}
	}

	return &RegisterResponse{UserID: userID}, nil
}

// GetProfile gets the current user's profile
//encore:api auth method=GET path=/user.profile
func GetProfile(ctx context.Context) (*User, error) {
	userID, _ := auth.UserID()

	var user User
	err := usersDB.QueryRow(ctx, `
		SELECT id, email, name, created_at
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "user not found",
		}
	}

	return &user, nil
}

// UpdateProfile updates the current user's profile
//encore:api auth method=PUT path=/user.profile
func UpdateProfile(ctx context.Context, params *struct {
	Name string `json:"name"`
}) (*User, error) {
	userID, _ := auth.UserID()

	var user User
	err := usersDB.QueryRow(ctx, `
		UPDATE users
		SET name = $1
		WHERE id = $2
		RETURNING id, email, name, created_at
	`, params.Name, userID).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "user not found",
		}
	}

	return &user, nil
}

// hashPassword is a placeholder for password hashing
func hashPassword(password string) string {
	// In production, use bcrypt or Argon2
	return "hashed_" + password
}

// usersDB is the database for user data
var usersDB = sqldb.NewDatabase("users", sqldb.DatabaseConfig{
	// Connection details will be injected by Infracast
})
