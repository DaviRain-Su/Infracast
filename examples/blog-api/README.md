# Blog API Example

A blog API demonstrating Infracast with 2 services, PostgreSQL, and OSS.

## Features

- **Post Service**: Blog post CRUD, listing, search
- **User Service**: User registration, authentication
- **Uploads**: Image uploads to OSS for blog posts

## Infrastructure Resources

| Resource | Type | Description |
|----------|------|-------------|
| blog-db | PostgreSQL | Blog posts and user data |
| uploads | OSS | Blog post images |

## Project Structure

```
blog-api/
├── post/              # Post service
│   └── post.go        # Blog post APIs
├── user/              # User service
│   └── user.go        # User management
├── uploads/           # Upload service
│   └── upload.go      # Image upload to OSS
├── migrations/        # Database migrations
│   └── 001_init.sql
├── encore.app         # Encore app configuration
└── README.md          # This file
```

## Quick Start

```bash
# Initialize project
infracast init blog-demo --provider alicloud --region cn-hangzhou

# Configure resources
# Edit infracast.yaml to add sql_servers.blog-db and object_storage.uploads

# Provision infrastructure
infracast provision --env dev

# Deploy application
infracast deploy --env dev
```

## API Endpoints

### Post Service

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /post.create | Create blog post |
| GET | /post.get | Get single post |
| GET | /post.list | List posts |
| POST | /post.search | Search posts |

### User Service

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /user.register | Register user |
| POST | /user.login | User login |
| GET | /user.profile | Get profile |

### Upload Service

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /upload.image | Upload image to OSS |
