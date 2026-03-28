-- sqlc reads the special comments above each statement to generate
-- strongly-typed Go functions. The format is:
--   -- name: <FunctionName> :<one|many|exec|execresult>

-- name: GetUser :one
SELECT * FROM users
WHERE id = $1
LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC;

-- name: CreateUser :one
INSERT INTO users (name, email)
VALUES ($1, $2)
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;
