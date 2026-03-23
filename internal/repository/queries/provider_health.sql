-- name: UpsertHealth :exec
INSERT INTO provider_health (provider_id, currency_pair_id, last_success_at, last_failure_at, consecutive_failures, last_error_message, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (provider_id, currency_pair_id)
    DO UPDATE SET
                  last_success_at      = COALESCE(EXCLUDED.last_success_at, provider_health.last_success_at),
                  last_failure_at      = COALESCE(EXCLUDED.last_failure_at, provider_health.last_failure_at),
                  consecutive_failures = EXCLUDED.consecutive_failures,
                  last_error_message   = EXCLUDED.last_error_message,
                  updated_at           = now();

-- name: RecordSuccess :exec
INSERT INTO provider_health (provider_id, currency_pair_id, last_success_at, consecutive_failures, last_error_message, updated_at)
VALUES ($1, $2, now(), 0, NULL, now())
ON CONFLICT (provider_id, currency_pair_id)
    DO UPDATE SET
                  last_success_at      = now(),
                  consecutive_failures = 0,
                  last_error_message   = NULL,
                  updated_at           = now();

-- name: RecordFailure :exec
INSERT INTO provider_health (provider_id, currency_pair_id, last_failure_at, consecutive_failures, last_error_message, updated_at)
VALUES ($1, $2, now(), 1, $3, now())
ON CONFLICT (provider_id, currency_pair_id)
    DO UPDATE SET
                  last_failure_at      = now(),
                  consecutive_failures = provider_health.consecutive_failures + 1,
                  last_error_message   = $3,
                  updated_at           = now();

-- name: GetHealthByProvider :many
SELECT id, provider_id, currency_pair_id, last_success_at, last_failure_at, consecutive_failures, last_error_message, updated_at
FROM provider_health
WHERE provider_id = $1
ORDER BY currency_pair_id;

-- name: GetAllHealth :many
SELECT
    ph.id,
    ph.provider_id,
    ph.currency_pair_id,
    ph.last_success_at,
    ph.last_failure_at,
    ph.consecutive_failures,
    ph.last_error_message,
    ph.updated_at,
    p.name AS provider_name
FROM provider_health ph
         JOIN providers p ON p.id = ph.provider_id
ORDER BY p.name, ph.currency_pair_id;