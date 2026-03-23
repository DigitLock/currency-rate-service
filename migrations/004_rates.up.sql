CREATE TABLE rates (
                       id                 BIGSERIAL     PRIMARY KEY,
                       currency_pair_id   BIGINT        NOT NULL,
                       source_provider_id BIGINT        NOT NULL,
                       rate               NUMERIC(20,10) NOT NULL,
                       is_outdated        BOOLEAN       NOT NULL DEFAULT false,
                       fetched_at         TIMESTAMPTZ   NOT NULL,
                       created_at         TIMESTAMPTZ   NOT NULL DEFAULT now(),

                       CONSTRAINT rates_rate_check     CHECK (rate > 0),
                       CONSTRAINT rates_pair_fk     FOREIGN KEY (currency_pair_id)   REFERENCES currency_pairs(id),
                       CONSTRAINT rates_provider_fk FOREIGN KEY (source_provider_id) REFERENCES providers(id)
);

-- Efficient lookup of the most recent rate per pair
CREATE INDEX rates_latest_idx  ON rates (currency_pair_id, fetched_at DESC);

-- Efficient range queries for GetRateHistory
CREATE INDEX rates_history_idx ON rates (currency_pair_id, fetched_at ASC);