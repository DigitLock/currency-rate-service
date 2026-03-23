CREATE TABLE currency_pairs (
                                id            BIGSERIAL    PRIMARY KEY,
                                from_currency VARCHAR(10)  NOT NULL,
                                to_currency   VARCHAR(10)  NOT NULL,
                                polling_interval_seconds INTEGER NOT NULL DEFAULT 3600,
                                is_active     BOOLEAN      NOT NULL DEFAULT true,
                                created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
                                updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

                                CONSTRAINT currency_pairs_pair_unique   UNIQUE (from_currency, to_currency),
                                CONSTRAINT currency_pairs_interval_check CHECK (polling_interval_seconds >= 60),
                                CONSTRAINT currency_pairs_from_check    CHECK (from_currency ~ '^[A-Z]{3,5}$'),
                                CONSTRAINT currency_pairs_to_check      CHECK (to_currency ~ '^[A-Z]{3,5}$')
);

-- Seed data: v1 currency pairs (SRS 5.4)
INSERT INTO currency_pairs (from_currency, to_currency, polling_interval_seconds, is_active)
VALUES
    ('RSD', 'EUR', 3600, true),
    ('RSD', 'USD', 3600, true),
    ('EUR', 'USD', 3600, true);