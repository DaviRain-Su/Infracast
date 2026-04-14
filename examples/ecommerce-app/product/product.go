// Package product provides product catalog APIs
package product

import (
	"context"
	"time"

	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
)

// Product represents a product in the catalog
type Product struct {
	ID             int       `json:"id"`
	SKU            string    `json:"sku"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Price          float64   `json:"price"`
	InventoryCount int       `json:"inventory_count"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListParams contains list parameters
type ListParams struct {
	Page     int `json:"page" query:"page"`
	PageSize int `json:"page_size" query:"page_size"`
}

// ListResponse contains list response
type ListResponse struct {
	Products   []Product `json:"products"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
}

// List returns a list of products
//encore:api public method=GET path=/product.list
func List(ctx context.Context, params *ListParams) (*ListResponse, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	// Query products
	offset := (params.Page - 1) * params.PageSize
	rows, err := productsDB.Query(ctx, `
		SELECT id, sku, name, description, price, inventory_count, created_at
		FROM products
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, params.PageSize, offset)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to query products",
		}
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.Price, &p.InventoryCount, &p.CreatedAt)
		if err != nil {
			continue
		}
		products = append(products, p)
	}

	// Get total count
	var total int
	err = productsDB.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&total)
	if err != nil {
		total = len(products)
	}

	return &ListResponse{
		Products: products,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// GetParams contains get parameters
type GetParams struct {
	SKU string `json:"sku" query:"sku"`
}

// Get returns a single product by SKU
//encore:api public method=GET path=/product.get
func Get(ctx context.Context, params *GetParams) (*Product, error) {
	if params.SKU == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "SKU is required",
		}
	}

	var product Product
	err := productsDB.QueryRow(ctx, `
		SELECT id, sku, name, description, price, inventory_count, created_at
		FROM products
		WHERE sku = $1
	`, params.SKU).Scan(&product.ID, &product.SKU, &product.Name, &product.Description, &product.Price, &product.InventoryCount, &product.CreatedAt)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "product not found",
		}
	}

	return &product, nil
}

// SearchParams contains search parameters
type SearchParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// Search searches products by name or description
//encore:api public method=POST path=/product.search
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

	rows, err := productsDB.Query(ctx, `
		SELECT id, sku, name, description, price, inventory_count, created_at
		FROM products
		WHERE name ILIKE $1 OR description ILIKE $1
		ORDER BY name
		LIMIT $2
	`, "%"+params.Query+"%", params.Limit)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "search failed",
		}
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.Price, &p.InventoryCount, &p.CreatedAt)
		if err != nil {
			continue
		}
		products = append(products, p)
	}

	return &ListResponse{
		Products: products,
		Total:    len(products),
		Page:     1,
		PageSize: params.Limit,
	}, nil
}

// CheckInventory checks product inventory
//encore:api public method=GET path=/product.inventory
func CheckInventory(ctx context.Context, params *GetParams) (*struct {
	SKU            string `json:"sku"`
	InventoryCount int    `json:"inventory_count"`
	Available      bool   `json:"available"`
}, error) {
	product, err := Get(ctx, params)
	if err != nil {
		return nil, err
	}

	return &struct {
		SKU            string `json:"sku"`
		InventoryCount int    `json:"inventory_count"`
		Available      bool   `json:"available"`
	}{
		SKU:            product.SKU,
		InventoryCount: product.InventoryCount,
		Available:      product.InventoryCount > 0,
	}, nil
}

// productsDB is the database for product data
var productsDB = sqldb.NewDatabase("products", sqldb.DatabaseConfig{
	// Connection details will be injected by Infracast
})
