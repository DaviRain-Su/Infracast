// Package post provides blog post APIs
package post

import (
	"context"
	"strconv"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
)

// Post represents a blog post
type Post struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	AuthorID  int       `json:"author_id"`
	ImageURL  string    `json:"image_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateParams contains post creation parameters
type CreateParams struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	ImageURL string `json:"image_url,omitempty"`
}

// Create creates a new blog post
//encore:api auth method=POST path=/post.create
func Create(ctx context.Context, params *CreateParams) (*Post, error) {
	userID, _ := auth.UserID()
	uid, _ := strconv.ParseInt(string(userID), 10, 64)
	authorID := int(uid)

	if params.Title == "" || params.Content == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "title and content are required",
		}
	}

	var post Post
	err := blogDB.QueryRow(ctx, `
		INSERT INTO posts (title, content, author_id, image_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, title, content, author_id, image_url, created_at, updated_at
	`, params.Title, params.Content, authorID, params.ImageURL).Scan(
		&post.ID, &post.Title, &post.Content, &post.AuthorID, &post.ImageURL, &post.CreatedAt, &post.UpdatedAt,
	)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to create post",
		}
	}

	return &post, nil
}

// GetParams contains get parameters
type GetParams struct {
	ID int `json:"id" query:"id"`
}

// Get returns a single blog post
//encore:api public method=GET path=/post.get
func Get(ctx context.Context, params *GetParams) (*Post, error) {
	if params.ID <= 0 {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "post ID is required",
		}
	}

	var post Post
	err := blogDB.QueryRow(ctx, `
		SELECT id, title, content, author_id, image_url, created_at, updated_at
		FROM posts
		WHERE id = $1
	`, params.ID).Scan(&post.ID, &post.Title, &post.Content, &post.AuthorID, &post.ImageURL, &post.CreatedAt, &post.UpdatedAt)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "post not found",
		}
	}

	return &post, nil
}

// ListParams contains list parameters
type ListParams struct {
	Page     int `json:"page" query:"page"`
	PageSize int `json:"page_size" query:"page_size"`
}

// ListResponse contains list response
type ListResponse struct {
	Posts    []Post `json:"posts"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// List returns a list of blog posts
//encore:api public method=GET path=/post.list
func List(ctx context.Context, params *ListParams) (*ListResponse, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	offset := (params.Page - 1) * params.PageSize

	rows, err := blogDB.Query(ctx, `
		SELECT id, title, content, author_id, image_url, created_at, updated_at
		FROM posts
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, params.PageSize, offset)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to query posts",
		}
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.ImageURL, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			continue
		}
		posts = append(posts, p)
	}

	var total int
	_ = blogDB.QueryRow(ctx, `SELECT COUNT(*) FROM posts`).Scan(&total)

	return &ListResponse{
		Posts:    posts,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// SearchParams contains search parameters
type SearchParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// Search searches blog posts
//encore:api public method=POST path=/post.search
func Search(ctx context.Context, params *SearchParams) (*ListResponse, error) {
	if params.Query == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "search query is required",
		}
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}

	rows, err := blogDB.Query(ctx, `
		SELECT id, title, content, author_id, image_url, created_at, updated_at
		FROM posts
		WHERE title ILIKE $1 OR content ILIKE $1
		ORDER BY created_at DESC
		LIMIT $2
	`, "%"+params.Query+"%", params.Limit)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "search failed",
		}
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.AuthorID, &p.ImageURL, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			continue
		}
		posts = append(posts, p)
	}

	return &ListResponse{
		Posts: posts,
		Total: len(posts),
		Page:  1,
		PageSize: params.Limit,
	}, nil
}

// blogDB is the database for blog data
var blogDB = sqldb.NewDatabase("blog", sqldb.DatabaseConfig{})
