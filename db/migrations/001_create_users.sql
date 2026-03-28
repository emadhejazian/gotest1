-- Migration: 001_create_users
-- Run this against your database before starting the server.
-- In a real project you would use a migration tool like golang-migrate
-- so that migrations are versioned, tracked, and run automatically.

CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL    PRIMARY KEY,
    name       TEXT         NOT NULL,
    email      TEXT         NOT NULL UNIQUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
