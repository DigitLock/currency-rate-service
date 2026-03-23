-- name: GetProviderByID :one
SELECT id, name, adapter_type, base_url, rate_json_path, currency_code_mapping, api_key, is_active, created_at, updated_at
FROM providers
WHERE id = $1;

-- name: GetActiveProviders :many
SELECT id, name, adapter_type, base_url, rate_json_path, currency_code_mapping, api_key, is_active, created_at, updated_at
FROM providers
WHERE is_active = true
ORDER BY id;