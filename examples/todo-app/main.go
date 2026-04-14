// Example API Application for Infracast
// This demonstrates a simple REST API service with database and cache integration
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"encore.dev/storage/sqldb"
	"encore.dev/storage/cache"
)

// User represents a user in the system
type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// usersDB is the database connection (managed by Infracast)
var usersDB = sqldb.NewDatabase("users", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// sessionCache is the cache connection (managed by Infracast)
var sessionCache = cache.NewCluster("session", cache.ClusterConfig{
	EvictionPolicy: cache.AllKeysLRU,
})

// GetUser retrieves a user by ID
//encore:api public path=/users/:id
func GetUser(ctx context.Context, id int64) (*User, error) {
	// Try cache first
	var user User
	cached, err := sessionCache.Get(ctx, fmt.Sprintf("user:%d", id))
	if err == nil && cached != "" {
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			return &user, nil
		}
	}

	// Fetch from database
	row := usersDB.QueryRow(ctx, "SELECT id, name, email FROM users WHERE id = $1", id)
	if err := row.Scan(&user.ID, &user.Name, &user.Email); err != nil {
		return nil, err
	}

	// Cache for future requests
	if data, err := json.Marshal(user); err == nil {
		sessionCache.Set(ctx, fmt.Sprintf("user:%d", id), string(data), 300) // 5 min TTL
	}

	return &user, nil
}

// CreateUser creates a new user
//encore:api public method=POST path=/users
func CreateUser(ctx context.Context, req *User) (*User, error) {
	row := usersDB.QueryRow(ctx,
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		req.Name, req.Email,
	)
	if err := row.Scan(&req.ID); err != nil {
		return nil, err
	}
	return req, nil
}

// HealthCheck returns the service health status
//encore:api public path=/health
func HealthCheck(ctx context.Context) (*HealthResponse, error) {
	return &HealthResponse{
		Status:  "ok",
		Version: os.Getenv("APP_VERSION"),
	}, nil
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// ListUsers returns all users
//encore:api public path=/users
func ListUsers(ctx context.Context) (*ListUsersResponse, error) {
	rows, err := usersDB.Query(ctx, "SELECT id, name, email FROM users LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return &ListUsersResponse{Users: users}, nil
}

// ListUsersResponse represents the list users response
type ListUsersResponse struct {
	Users []User `json:"users"`
}

func main() {
	log.Println("API App starting...")
	// Encore handles the server lifecycle
}
