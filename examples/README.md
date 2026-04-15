# Infracast Example Applications

This directory contains example applications demonstrating Infracast deployment capabilities.

## Applications

### 1. hello-world (Zero Resources)

The simplest possible Infracast application demonstrating:
- **Zero cloud resources** (no database, cache, or storage)
- Basic HTTP service with Encore
- Health check endpoint
- Perfect for onboarding and first deployment

**Endpoints:**
- `GET /hello` - Returns greeting message
- `GET /health` - Health check

**Resources:** None (build → deploy only)

### 2. todo-app (REST API Service)

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

### 3. web-app (Web Frontend)

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

### 5. health-check (Minimal Observable Application)

A minimal observable application demonstrating:
- **Liveness probe** (`/livez`) — always returns ok + uptime
- **Readiness probe** (`/readyz`) — dependency checks with failure simulation
- **Diagnostics** (`/diag`) — hostname, Go version, CPU count, environment
- **Zero cloud resources** — no database, cache, or storage
- **Failure mode** — set `SIMULATE_FAILURE=true` to test failure detection

**Endpoints:**
- `GET /livez` - Kubernetes liveness probe
- `GET /readyz` - Kubernetes readiness probe (with dependency checks)
- `GET /diag` - Diagnostic information

**Resources:** None

**Use case:** Verify Infracast deploy pipeline end-to-end, including health verification and failure detection.

### 6. migration (Database Migrations)

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
# Deploy hello-world (zero resources)
cd hello-world
infracast deploy --env staging

# Deploy todo-app (with database + cache)
cd todo-app
infracast deploy --env staging

# Deploy web-app (with object storage)
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
cd hello-world
infracast run
```

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  hello-world    │     │   todo-app      │     │   web-app       │
│ (Zero Resources)│     │   (REST API)    │     │   (Web Frontend)│
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         │                  ┌────┴────┐             ┌────┴────┐
         │                  │         │             │         │
         │              ┌───▼───┐ ┌───▼───┐     ┌───▼───┐ ┌───▼────┐
         │              │PostgreSQL│ │ Redis │     │  OSS  │ │  CDN   │
         │              └─────────┘ └───────┘     └───────┘ └────────┘
         │
    No external resources needed
```

## Notes

- All apps use Encore framework annotations (`//encore:api`)
- Resources are provisioned automatically by Infracast
- Configuration is generated in `infracfg.json`
- Forward-only migrations ensure safe deployments
- **hello-world** requires no cloud resources - perfect for first deployment
