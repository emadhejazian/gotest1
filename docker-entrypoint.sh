#!/bin/sh
set -e

echo "Waiting for postgres to be ready..."
until PGPASSWORD=gotest1 psql -h postgres -U gotest1 -d gotest1 -c '\q' 2>/dev/null; do
  sleep 1
done

echo "Running database migrations..."
PGPASSWORD=gotest1 psql -h postgres -U gotest1 -d gotest1 -f /root/db/migrations/001_create_users.sql

echo "Starting API server..."
exec ./api
