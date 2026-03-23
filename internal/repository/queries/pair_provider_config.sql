-- name: GetProvidersForPair :many
SELECT
    ppc.id,
    ppc.currency_pair_id,
    ppc.provider_id,
    ppc.priority,
    ppc.is_active,
    ppc.created_at,
    p.name AS provider_name,
    p.adapter_type,
    p.base_url,
    p.rate_json_path,
    p.currency_code_mapping,
    p.api_key
FROM pair_provider_config ppc
         JOIN providers p ON p.id = ppc.provider_id
WHERE ppc.currency_pair_id = $1
  AND ppc.is_active = true
  AND p.is_active = true
ORDER BY
    CASE ppc.priority WHEN 'primary' THEN 0 ELSE 1 END;