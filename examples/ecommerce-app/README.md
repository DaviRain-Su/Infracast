# E-commerce Example App

A full-featured e-commerce application demonstrating Infracast's capabilities with multiple resources and services.

## Features

- **User Service**: User registration, authentication, profile management
- **Product Service**: Product catalog, search, inventory management
- **Order Service**: Shopping cart, checkout, order history
- **Notification Service**: Email and SMS notifications

## Infrastructure Resources

| Resource | Type | Description |
|----------|------|-------------|
| users-db | PostgreSQL | User data and authentication |
| products-db | PostgreSQL | Product catalog and inventory |
| orders-db | PostgreSQL | Order and transaction data |
| session-cache | Redis | Session storage and rate limiting |
| inventory-cache | Redis | Inventory cache and product search |
| assets | OSS | Product images and static assets |

## Project Structure

```
ecommerce-app/
├── user/               # User service
│   ├── user.go        # User management API
│   └── auth.go        # Authentication
├── product/           # Product service
│   ├── product.go     # Product catalog API
│   └── inventory.go   # Inventory management
├── order/             # Order service
│   ├── cart.go        # Shopping cart
│   ├── checkout.go    # Checkout process
│   └── order.go       # Order management
├── notify/            # Notification service
│   └── notify.go      # Email/SMS notifications
├── migrations/        # Database migrations
│   ├── 001_init.sql
│   └── 002_add_orders.sql
└── encore.app         # Encore app configuration
```

## Quick Start

### 1. Initialize Project

```bash
infracast init ecommerce-demo --provider alicloud --region cn-hangzhou
```

### 2. Configure Resources

Edit `infracast.yaml`:

```yaml
app_name: ecommerce-demo
provider: alicloud
region: cn-hangzhou

environments:
  dev:
    description: Development environment
  
  staging:
    description: Staging environment
  
  production:
    description: Production environment

resources:
  sql_servers:
    users-db:
      instance_class: pg.n2.medium.1
      storage: 50
    products-db:
      instance_class: pg.n2.medium.1
      storage: 100
    orders-db:
      instance_class: pg.n2.large.1
      storage: 200
  
  redis:
    session-cache:
      node_type: redis.master.small.default
    inventory-cache:
      node_type: redis.master.mid.default
  
  object_storage:
    assets:
      storage_class: STANDARD
      bucket_name: ecommerce-assets-demo
```

### 3. Provision Infrastructure

```bash
infracast provision --env dev
```

### 4. Deploy Application

```bash
infracast deploy --env dev
```

### 5. Run Migration

```bash
infracast migrate --env dev
```

## API Endpoints

### User Service

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /user.register | Register new user |
| POST | /user.login | User login |
| GET | /user.profile | Get user profile |
| PUT | /user.profile | Update profile |

### Product Service

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /product.list | List products |
| GET | /product.get | Get product details |
| POST | /product.search | Search products |
| GET | /product.inventory | Check inventory |

### Order Service

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /cart.add | Add to cart |
| GET | /cart.get | Get cart contents |
| POST | /order.checkout | Checkout |
| GET | /order.list | List orders |
| GET | /order.get | Get order details |

## Database Schema

### users table
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);
```

### products table
```sql
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    sku VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    inventory_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### orders table
```sql
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    status VARCHAR(50) DEFAULT 'pending',
    total_amount DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

## Deployment Manual

See [DEPLOYMENT.md](./DEPLOYMENT.md) for detailed deployment instructions.

## Architecture

```
┌─────────────────┐
│   API Gateway   │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
┌───▼───┐ ┌───▼────┐
│ User  │ │ Product│
│Service│ │Service │
└───┬───┘ └────┬───┘
    │          │
┌───▼───┐ ┌────▼───┐
│users  │ │products│
│  DB   │ │   DB   │
└───────┘ └────────┘
    │
┌───▼───┐ ┌────────┐
│ Order │ │ Notify │
│Service│ │Service │
└───┬───┘ └────────┘
    │
┌───▼───┐
│orders │
│  DB   │
└───────┘
```
