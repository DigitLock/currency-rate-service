CREATE TABLE providers (
                           id                     BIGSERIAL    PRIMARY KEY,
                           name                   VARCHAR(100) NOT NULL,
                           adapter_type           VARCHAR(20)  NOT NULL,
                           base_url               TEXT         NOT NULL,
                           rate_json_path         VARCHAR(255),
                           currency_code_mapping  JSONB,
                           api_key                VARCHAR(255),
                           is_active              BOOLEAN      NOT NULL DEFAULT true,
                           created_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
                           updated_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),

                           CONSTRAINT providers_name_unique        UNIQUE (name),
                           CONSTRAINT providers_adapter_type_check CHECK (adapter_type IN ('generic_json', 'custom'))
);

-- Seed data: v1 providers (SRS 5.4)
INSERT INTO providers (name, adapter_type, base_url, rate_json_path, currency_code_mapping, api_key, is_active)
VALUES
    (
        'fawazahmed0',
        'generic_json',
        'https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/{from}.min.json',
        '{from}.{to}',
        '{"RSD": "rsd", "EUR": "eur", "USD": "usd"}',
        NULL,
        true
    ),
    (
        'fawazahmed0-fallback',
        'generic_json',
        'https://latest.currency-api.pages.dev/v1/currencies/{from}.min.json',
        '{from}.{to}',
        '{"RSD": "rsd", "EUR": "eur", "USD": "usd"}',
        NULL,
        true
    ),
    (
        'exchangerate-api',
        'generic_json',
        'https://open.er-api.com/v6/latest/{from}',
        'rates.{to}',
        NULL,
        NULL,
        true
    ),
    (
        'frankfurter',
        'generic_json',
        'https://api.frankfurter.dev/v1/latest?base={from}&symbols={to}',
        'rates.{to}',
        NULL,
        NULL,
        true
    );