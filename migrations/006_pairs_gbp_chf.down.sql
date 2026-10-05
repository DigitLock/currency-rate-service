-- Rolling back deletes the rate history of GBP→USD and CHF→USD.

BEGIN;

DELETE FROM provider_health
WHERE currency_pair_id IN (
    SELECT id FROM currency_pairs
    WHERE (from_currency, to_currency) IN (('GBP', 'USD'), ('CHF', 'USD'))
);

DELETE FROM rates
WHERE currency_pair_id IN (
    SELECT id FROM currency_pairs
    WHERE (from_currency, to_currency) IN (('GBP', 'USD'), ('CHF', 'USD'))
);

DELETE FROM pair_provider_config
WHERE currency_pair_id IN (
    SELECT id FROM currency_pairs
    WHERE (from_currency, to_currency) IN (('GBP', 'USD'), ('CHF', 'USD'))
);

DELETE FROM currency_pairs
WHERE (from_currency, to_currency) IN (('GBP', 'USD'), ('CHF', 'USD'));

UPDATE providers
SET currency_code_mapping = currency_code_mapping - 'GBP' - 'CHF',
    updated_at            = now()
WHERE name IN ('fawazahmed0', 'fawazahmed0-fallback');

COMMIT;
