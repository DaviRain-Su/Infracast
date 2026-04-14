# Infracast Example Applications

This directory contains example applications demonstrating Infracast deployment capabilities.

## Applications

### 1. api-app (REST API Service)

A simple REST API demonstrating:
- Database integration (PostgreSQL via `sqldb.NewDatabase`)
- Cache integration (Redis via `cache.NewCluster`)
- CRUD operations with caching
- Health check endpoint

**Endpoints:**
- `GET /users` - List all users
- `GET /users/:id` - Get user by ID (with cache)
- `POST /users` - Create new user
- `GET /health` - Health check

**Resources:**
- Database: `users` (PostgreSQL)
- Cache: `session` (Redis)

### 2. web-app (Web Frontend)

A web frontend demonstrating:
- Object storage integration (OSS via `objects.NewBucket`)
- File upload/download
- HTML rendering

**Endpoints:**
- `GET /` - Home page
- `POST /upload` - Upload file to object storage
- `GET /assets` - List stored assets
- `GET /health` - Health check

**Resources:**
- Object Storage: `assets` (OSS)

### 3. migration (Database Migrations)

Example database migrations demonstrating:
- Forward-only migration strategy
- Safe schema changes (adding columns, indexes)
- No destructive DDL in production

**Migrations:**
- `001_initial_schema.sql` - Create users and sessions tables
- `002_add_user_profile.sql` - Add avatar, bio, timezone columns

## Deployment

Deploy these examples with Infracast:

```bash
# Deploy api-app
cd api-app
infracast deploy --env staging

# Deploy web-app
cd web-app
infracast deploy --env staging

# Run migrations
infracast migrate --env staging
```

## Local Development

Run locally with Infracast:

```bash
# Start local development environment
infracast run

# Or with specific app
cd api-app
infracast run
```

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│   api-app       │     │   web-app       │
│   (REST API)    │     │   (Web Frontend)│
└────────┬────────┘     └────────┬────────┘
         │                       │
    ┌────┴────┐             ┌────┴────┐
    │         │             │         │
┌───▼───┐ ┌───▼───┐     ┌───▼───┐ ┌───▼────┐
│PostgreSQL│ │ Redis │     │  OSS  │ │  CDN   │
└─────────┘ └───────┘     └───────┘ └────────┘
```

## Notes

- All apps use Encore framework annotations (`//encore:api`)
- Resources are provisioned automatically by Infracast
- Configuration is generated in `infracfg.json`
- Forward-only migrations ensure safe deployments
