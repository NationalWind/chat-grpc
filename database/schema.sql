-- Database schema for chat application
-- PostgreSQL 12+

-- Create database (run as postgres superuser)
-- CREATE DATABASE chatapp;

-- Connect to chatapp database
-- \c chatapp;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_trgm; -- For fuzzy text search
CREATE EXTENSION IF NOT EXISTS btree_gin; -- For efficient indexing

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL, -- Hashed password
    display_name VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    is_online BOOLEAN DEFAULT FALSE
);

-- Groups table
CREATE TABLE IF NOT EXISTS groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Group members table (many-to-many relationship)
CREATE TABLE IF NOT EXISTS group_members (
    id SERIAL PRIMARY KEY,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    username VARCHAR(50) NOT NULL REFERENCES users(username) ON DELETE CASCADE,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(group_id, username)
);

-- Messages table
CREATE TABLE IF NOT EXISTS messages (
    id SERIAL PRIMARY KEY,
    from_user VARCHAR(50) NOT NULL,
    to_target VARCHAR(100) NOT NULL, -- username for private, group name for group
    message_type VARCHAR(20) NOT NULL, -- 'private' or 'group'
    text TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for efficient searching
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_username_trgm ON users USING gin(username gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_users_display_name_trgm ON users USING gin(display_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_users_online ON users(is_online);
CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
CREATE INDEX IF NOT EXISTS idx_group_members_group ON group_members(group_id);
CREATE INDEX IF NOT EXISTS idx_group_members_username ON group_members(username);
CREATE INDEX IF NOT EXISTS idx_messages_from ON messages(from_user);
CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_target);
CREATE INDEX IF NOT EXISTS idx_messages_type ON messages(message_type);

-- Function to search users (case-insensitive, fuzzy)
CREATE OR REPLACE FUNCTION search_users(search_query TEXT)
RETURNS TABLE (
    username VARCHAR(50),
    display_name VARCHAR(100),
    is_online BOOLEAN,
    similarity_score REAL
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        u.username,
        u.display_name,
        u.is_online,
        GREATEST(
            similarity(LOWER(u.username), LOWER(search_query)),
            similarity(LOWER(COALESCE(u.display_name, '')), LOWER(search_query))
        ) as similarity_score
    FROM users u
    WHERE
        LOWER(u.username) LIKE LOWER('%' || search_query || '%')
        OR LOWER(u.display_name) LIKE LOWER('%' || search_query || '%')
        OR similarity(LOWER(u.username), LOWER(search_query)) > 0.3
        OR similarity(LOWER(COALESCE(u.display_name, '')), LOWER(search_query)) > 0.3
    ORDER BY similarity_score DESC, u.is_online DESC, u.username ASC
    LIMIT 20;
END;
$$ LANGUAGE plpgsql;

-- Sample data for testing
-- INSERT INTO users (username, display_name) VALUES
--     ('alice', 'Alice Wonderland'),
--     ('bob', 'Bob Builder'),
--     ('charlie', 'Charlie Brown'),
--     ('david', 'David Davidson'),
--     ('eve', 'Eve Online');

-- Useful queries:
-- Search users: SELECT * FROM search_users('ali');
-- Get all online users: SELECT * FROM users WHERE is_online = TRUE;
-- Update user status: UPDATE users SET is_online = TRUE, last_seen = NOW() WHERE username = 'alice';
