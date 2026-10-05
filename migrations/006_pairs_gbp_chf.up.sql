-- Pairs GBP→USD and CHF→USD with provider assignments (SRS 5.4, BRD Amendment 1)
-- One transaction: the service hot-reloads configuration (SRS 3.1.2) and must see
-- the mapping, pairs and assignments together.

BEGIN;

-- Currency code mapping for both fawazahmed0 providers
UPDATE providers
SET currency_code_mapping = currency_code_mapping || '{"GBP": "gbp", "CHF": "chf"}'::jsonb,
    updated_at            = now()
WHERE name IN ('fawazahmed0', 'fawazahmed0-fallback');

INSERT INTO currency_pairs (from_currency, to_currency, polling_interval_seconds, is_active)
VALUES
    ('GBP', 'USD', 3600, true),
    ('CHF', 'USD', 3600, true);

-- GBP→USD: primary=frankfurter, backup=fawazahmed0-fallback
-- CHF→USD: primary=frankfurter, backup=fawazahmed0-fallback
INSERT INTO pair_provider_config (currency_pair_id, provider_id, priority, is_active)
SELECT cp.id, p.id, v.priority, true
FROM (
    VALUES
        ('GBP', 'USD', 'frankfurter',          'primary'),
        ('GBP', 'USD', 'fawazahmed0-fallback', 'backup'),
        ('CHF', 'USD', 'frankfurter',          'primary'),
        ('CHF', 'USD', 'fawazahmed0-fallback', 'backup')
) AS v (from_currency, to_currency, provider_name, priority)
JOIN currency_pairs cp ON cp.from_currency = v.from_currency AND cp.to_currency = v.to_currency
JOIN providers      p  ON p.name = v.provider_name;

COMMIT;
