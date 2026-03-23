CREATE TABLE pair_provider_config (
                                      id               BIGSERIAL   PRIMARY KEY,
                                      currency_pair_id BIGINT      NOT NULL,
                                      provider_id      BIGINT      NOT NULL,
                                      priority         VARCHAR(10) NOT NULL,
                                      is_active        BOOLEAN     NOT NULL DEFAULT true,
                                      created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

                                      CONSTRAINT pair_provider_config_priority_check CHECK (priority IN ('primary', 'backup')),
                                      CONSTRAINT pair_provider_config_pair_fk     FOREIGN KEY (currency_pair_id) REFERENCES currency_pairs(id),
                                      CONSTRAINT pair_provider_config_provider_fk FOREIGN KEY (provider_id)      REFERENCES providers(id)
);

-- Partial unique index: one active primary and one active backup per pair
CREATE UNIQUE INDEX pair_provider_config_unique
    ON pair_provider_config (currency_pair_id, priority)
    WHERE is_active = true;

-- Seed data: v1 pair-provider assignments (SRS 5.4)
-- RSD→EUR: primary=fawazahmed0 (id=1), backup=fawazahmed0-fallback (id=2)
-- RSD→USD: primary=fawazahmed0 (id=1), backup=exchangerate-api (id=3)
-- EUR→USD: primary=frankfurter (id=4), backup=fawazahmed0 (id=1)
INSERT INTO pair_provider_config (currency_pair_id, provider_id, priority, is_active)
VALUES
    (1, 1, 'primary', true),
    (1, 2, 'backup',  true),
    (2, 1, 'primary', true),
    (2, 3, 'backup',  true),
    (3, 4, 'primary', true),
    (3, 1, 'backup',  true);