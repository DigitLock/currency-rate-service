CREATE TABLE provider_health (
                                 id                    BIGSERIAL   PRIMARY KEY,
                                 provider_id           BIGINT      NOT NULL,
                                 currency_pair_id      BIGINT      NOT NULL,
                                 last_success_at       TIMESTAMPTZ,
                                 last_failure_at       TIMESTAMPTZ,
                                 consecutive_failures  INTEGER     NOT NULL DEFAULT 0,
                                 last_error_message    TEXT,
                                 updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

                                 CONSTRAINT provider_health_unique        UNIQUE (provider_id, currency_pair_id),
                                 CONSTRAINT provider_health_failures_check CHECK (consecutive_failures >= 0),
                                 CONSTRAINT provider_health_provider_fk FOREIGN KEY (provider_id)      REFERENCES providers(id),
                                 CONSTRAINT provider_health_pair_fk     FOREIGN KEY (currency_pair_id) REFERENCES currency_pairs(id)
);