package polling

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/DigitLock/currency-rate-service/internal/adapter"
	"github.com/DigitLock/currency-rate-service/internal/repository"
)

// buildProviders creates adapters for active provider rows (SRS 5.2, 5.3).
// Custom and unknown adapter types are skipped with a warning.
func buildProviders(rows []repository.Provider, timeout time.Duration, logger *slog.Logger) []adapter.RateProvider {
	providers := make([]adapter.RateProvider, 0, len(rows))

	for _, p := range rows {
		switch p.AdapterType {
		case "generic_json":
			var codeMapping map[string]string
			if p.CurrencyCodeMapping != nil {
				codeMapping = parseCodeMapping(p.CurrencyCodeMapping, logger)
			}

			cfg := adapter.GenericJSONConfig{
				ProviderName:        p.Name,
				BaseURL:             p.BaseUrl,
				RateJSONPath:        stringFromPgText(p.RateJsonPath),
				CurrencyCodeMapping: codeMapping,
			}

			providers = append(providers, adapter.NewGenericJSONAdapter(cfg, timeout))
			logger.Info("registered provider", "name", p.Name, "type", p.AdapterType)

		case "custom":
			logger.Warn("custom adapter not implemented, skipping", "name", p.Name)

		default:
			logger.Warn("unknown adapter type, skipping", "name", p.Name, "type", p.AdapterType)
		}
	}

	return providers
}

func parseCodeMapping(data []byte, logger *slog.Logger) map[string]string {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		logger.Warn("failed to parse currency_code_mapping", "error", err)
		return nil
	}
	return m
}

func stringFromPgText(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

// providersFingerprint returns a deterministic hash over the provider fields that affect
// adapter construction. It is used only to decide whether the registry must be rebuilt.
func providersFingerprint(rows []repository.Provider) string {
	sorted := make([]repository.Provider, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	h := sha256.New()
	for _, p := range sorted {
		writeField(h, []byte(p.Name))
		writeField(h, []byte(p.AdapterType))
		writeField(h, []byte(p.BaseUrl))
		writeField(h, []byte(stringFromPgText(p.RateJsonPath)))
		writeField(h, p.CurrencyCodeMapping)
		if p.ApiKey.Valid && p.ApiKey.String != "" {
			writeField(h, []byte{1})
		} else {
			writeField(h, []byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeField writes a length-prefixed field so that adjacent fields cannot run into each other.
func writeField(h hash.Hash, b []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	h.Write(n[:])
	h.Write(b)
}

// planReconcile compares running pair loops with the active pairs from the database (SRS 3.1.2).
// start: active pairs not running; stop: running pairs no longer active;
// restart: running pairs whose currencies or polling interval changed. Output is ordered by pair ID.
func planReconcile(running map[int64]repository.CurrencyPair, desired []repository.CurrencyPair) (start []repository.CurrencyPair, stop []int64, restart []repository.CurrencyPair) {
	desiredIDs := make(map[int64]struct{}, len(desired))

	for _, d := range desired {
		desiredIDs[d.ID] = struct{}{}

		r, ok := running[d.ID]
		switch {
		case !ok:
			start = append(start, d)
		case r.FromCurrency != d.FromCurrency ||
			r.ToCurrency != d.ToCurrency ||
			r.PollingIntervalSeconds != d.PollingIntervalSeconds:
			restart = append(restart, d)
		}
	}

	for id := range running {
		if _, ok := desiredIDs[id]; !ok {
			stop = append(stop, id)
		}
	}

	sort.Slice(start, func(i, j int) bool { return start[i].ID < start[j].ID })
	sort.Slice(restart, func(i, j int) bool { return restart[i].ID < restart[j].ID })
	sort.Slice(stop, func(i, j int) bool { return stop[i] < stop[j] })

	return start, stop, restart
}
