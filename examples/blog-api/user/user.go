// Package user provides user management APIs
package user

import (
	"context"
	"strconv"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
)

// User represents a user
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

// Register creates a new user
//encore:api public method=POST path=/user.register
func Register(ctx context.Context, params *RegisterParams) (*RegisterResponse, error) {
	if params.Email == "" || params.Password == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "email and password are required",
		}
	}

	var userID int
	err := blogDB.QueryRow(ctx, `
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

// LoginParams contains login parameters
type LoginParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse contains login response
type LoginResponse struct {
	Token  string `json:"token"`
	UserID int    `json:"user_id"`
}

// Login authenticates a user
//encore:api public method=POST path=/user.login
func Login(ctx context.Context, params *LoginParams) (*LoginResponse, error) {
	if params.Email == "" || params.Password == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "email and password are required",
		}
	}

	var userID int
	var passwordHash string
	err := blogDB.QueryRow(ctx, `
		SELECT id, password_hash FROM users WHERE email = $1
	`, params.Email).Scan(&userID, &passwordHash)

	if err != nil || !checkPassword(params.Password, passwordHash) {
		return nil, &errs.Error{
			Code:    errs.Unauthenticated,
			Message: "invalid credentials",
		}
	}

	// In production, generate JWT token
	return &LoginResponse{
		Token:  "placeholder-token",
		UserID: userID,
	}, nil
}

// Profile returns the current user's profile
//encore:api auth method=GET path=/user.profile
func Profile(ctx context.Context) (*User, error) {
	userID, _ := auth.UserID()
	uid64, _ := strconv.ParseInt(string(userID), 10, 64)
	uid := int(uid64)

	var user User
	err := blogDB.QueryRow(ctx, `
		SELECT id, email, name, created_at FROM users WHERE id = $1
	`, uid).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "user not found",
		}
	}

	return &user, nil
}

// hashPassword hashes a password
func hashPassword(password string) string {
	return "hashed_" + password
}

// checkPassword verifies a password
func checkPassword(password, hash string) bool {
	return hash == "hashed_"+password
}

// blogDB is the database
var blogDB = sqldb.NewDatabase("blog", sqldb.DatabaseConfig{})
