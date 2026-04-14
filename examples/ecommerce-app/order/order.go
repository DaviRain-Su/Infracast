// Package order provides order management APIs
package order

import (
	"context"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/sqldb"
)

// Order represents an order in the system
type Order struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	Status      string    `json:"status"`
	TotalAmount float64   `json:"total_amount"`
	Items       []OrderItem `json:"items"`
	CreatedAt   time.Time `json:"created_at"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ID        int     `json:"id"`
	ProductID int     `json:"product_id"`
	SKU       string  `json:"sku"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// CheckoutParams contains checkout parameters
type CheckoutParams struct {
	Items []struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	} `json:"items"`
}

// CheckoutResponse contains checkout response
type CheckoutResponse struct {
	OrderID int     `json:"order_id"`
	Status  string  `json:"status"`
	Total   float64 `json:"total"`
}

// Checkout creates a new order
//encore:api auth method=POST path=/order.checkout
func Checkout(ctx context.Context, params *CheckoutParams) (*CheckoutResponse, error) {
	userID, _ := auth.UserID()
	uid := int(userID.Int64())

	if len(params.Items) == 0 {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "order must contain at least one item",
		}
	}

	// Calculate total and validate items
	var total float64
	var orderItems []OrderItem

	for _, item := range params.Items {
		if item.Quantity <= 0 {
			continue
		}

		// In real implementation, fetch product price from product service
		price := 10.0 // Placeholder
		total += price * float64(item.Quantity)

		orderItems = append(orderItems, OrderItem{
			SKU:      item.SKU,
			Quantity: item.Quantity,
			Price:    price,
		})
	}

	// Begin transaction
	tx, err := ordersDB.Begin(ctx)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to begin transaction",
		}
	}
	defer tx.Rollback()

	// Insert order
	var orderID int
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, status, total_amount, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id
	`, uid, "pending", total).Scan(&orderID)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to create order",
		}
	}

	// Insert order items
	for _, item := range orderItems {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, sku, quantity, price)
			VALUES ($1, $2, $3, $4)
		`, orderID, item.SKU, item.Quantity, item.Price)
		if err != nil {
			return nil, &errs.Error{
				Code:    errs.Internal,
				Message: "failed to add order items",
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to complete order",
		}
	}

	return &CheckoutResponse{
		OrderID: orderID,
		Status:  "pending",
		Total:   total,
	}, nil
}

// ListParams contains list parameters
type ListParams struct {
	Page     int `json:"page" query:"page"`
	PageSize int `json:"page_size" query:"page_size"`
}

// ListResponse contains list response
type ListResponse struct {
	Orders   []Order `json:"orders"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

// List returns a list of user's orders
//encore:api auth method=GET path=/order.list
func List(ctx context.Context, params *ListParams) (*ListResponse, error) {
	userID, _ := auth.UserID()
	uid := int(userID.Int64())

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}

	offset := (params.Page - 1) * params.PageSize

	// Query orders
	rows, err := ordersDB.Query(ctx, `
		SELECT id, user_id, status, total_amount, created_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, uid, params.PageSize, offset)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to query orders",
		}
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalAmount, &o.CreatedAt)
		if err != nil {
			continue
		}
		o.Items = []OrderItem{} // Initialize empty items
		orders = append(orders, o)
	}

	// Get total count
	var total int
	err = ordersDB.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id = $1`, uid).Scan(&total)
	if err != nil {
		total = len(orders)
	}

	return &ListResponse{
		Orders:   orders,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// GetParams contains get parameters
type GetParams struct {
	OrderID int `json:"order_id" query:"order_id"`
}

// Get returns a single order with items
//encore:api auth method=GET path=/order.get
func Get(ctx context.Context, params *GetParams) (*Order, error) {
	userID, _ := auth.UserID()
	uid := int(userID.Int64())

	if params.OrderID <= 0 {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "order ID is required",
		}
	}

	// Query order
	var order Order
	err := ordersDB.QueryRow(ctx, `
		SELECT id, user_id, status, total_amount, created_at
		FROM orders
		WHERE id = $1 AND user_id = $2
	`, params.OrderID, uid).Scan(&order.ID, &order.UserID, &order.Status, &order.TotalAmount, &order.CreatedAt)

	if err != nil {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "order not found",
		}
	}

	// Query order items
	rows, err := ordersDB.Query(ctx, `
		SELECT id, sku, quantity, price
		FROM order_items
		WHERE order_id = $1
	`, params.OrderID)
	if err != nil {
		return &order, nil // Return order without items
	}
	defer rows.Close()

	for rows.Next() {
		var item OrderItem
		err := rows.Scan(&item.ID, &item.SKU, &item.Quantity, &item.Price)
		if err != nil {
			continue
		}
		order.Items = append(order.Items, item)
	}

	return &order, nil
}

// ordersDB is the database for order data
var ordersDB = sqldb.NewDatabase("orders", sqldb.DatabaseConfig{
	// Connection details will be injected by Infracast
})
