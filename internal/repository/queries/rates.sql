-- name: InsertRate :one
INSERT INTO rates (currency_pair_id, source_provider_id, rate, is_outdated, fetched_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, currency_pair_id, source_provider_id, rate, is_outdated, fetched_at, created_at;

-- name: GetLatestRate :one
SELECT
    r.id,
    r.currency_pair_id,
    r.source_provider_id,
    r.rate,
    r.is_outdated,
    r.fetched_at,
    r.created_at,
    p.name AS source_provider_name
FROM rates r
         JOIN providers p ON p.id = r.source_provider_id
WHERE r.currency_pair_id = $1
ORDER BY r.fetched_at DESC
LIMIT 1;

-- name: MarkOutdated :exec
UPDATE rates
SET is_outdated = true
WHERE id = (
    SELECT r2.id FROM rates r2
    WHERE r2.currency_pair_id = $1
    ORDER BY r2.fetched_at DESC
    LIMIT 1
);

-- name: GetRateHistory :many
SELECT
    r.id,
    r.currency_pair_id,
    r.source_provider_id,
    r.rate,
    r.is_outdated,
    r.fetched_at,
    r.created_at,
    p.name AS source_provider_name
FROM rates r
         JOIN providers p ON p.id = r.source_provider_id
WHERE r.currency_pair_id = $1
  AND r.fetched_at >= $2
  AND r.fetched_at <= $3
ORDER BY r.fetched_at ASC;