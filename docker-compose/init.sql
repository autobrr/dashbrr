-- init.sql
--
-- This script initializes the PostgreSQL database for integration tests.
-- It is automatically executed when the test database container starts via
-- docker-compose.integration.yml.
--
-- The script:
-- 1. Creates a fresh test database
-- 2. Sets up the required schema
-- 3. Configures proper permissions
--
-- Usage: Referenced in docker-compose.integration.yml as a volume mount:
--   volumes:
--     - ./init.sql:/docker-entrypoint-initdb.d/init.sql

-- Drop the database if it exists and recreate it
DROP DATABASE IF EXISTS dashbrr_test;
CREATE DATABASE dashbrr_test;

-- Connect to the test database
\c dashbrr_test;

-- Create the necessary tables
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS service_configurations (
    id SERIAL PRIMARY KEY,
    instance_id TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    url TEXT,
    api_key TEXT,
    access_url TEXT
);

-- Grant all privileges to the dashbrr user
GRANT ALL PRIVILEGES ON DATABASE dashbrr_test TO dashbrr;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO dashbrr;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO dashbrr;
