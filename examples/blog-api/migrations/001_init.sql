-- Initial schema for blog API

-- Users table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Posts table
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    author_id INTEGER REFERENCES users(id),
    image_url TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_posts_author ON posts(author_id);
CREATE INDEX idx_posts_created ON posts(created_at);

-- Insert sample user
INSERT INTO users (email, password_hash, name) VALUES
('demo@example.com', 'hashed_demo123', 'Demo User');

-- Insert sample posts
INSERT INTO posts (title, content, author_id) VALUES
('Getting Started with Infracast', 'Infracast makes it easy to deploy Encore apps to the cloud...', 1),
('Best Practices for Cloud Infrastructure', 'When building cloud-native applications...', 1);
