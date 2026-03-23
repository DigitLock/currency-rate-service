-- name: GetActivePairs :many
SELECT id, from_currency, to_currency, polling_interval_seconds, is_active, created_at, updated_at
FROM currency_pairs
WHERE is_active = true
ORDER BY id;

-- name: FindPairByCode :one
SELECT id, from_currency, to_currency, polling_interval_seconds, is_active, created_at, updated_at
FROM currency_pairs
WHERE from_currency = $1 AND to_currency = $2 AND is_active = true;

-- name: GetAllPairs :many
SELECT id, from_currency, to_currency, polling_interval_seconds, is_active, created_at, updated_at
FROM currency_pairs
ORDER BY id;