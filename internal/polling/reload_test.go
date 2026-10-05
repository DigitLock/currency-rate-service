package polling

import (
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/DigitLock/currency-rate-service/internal/repository"
)

func pair(id int64, from, to string, interval int32) repository.CurrencyPair {
	return repository.CurrencyPair{
		ID:                     id,
		FromCurrency:           from,
		ToCurrency:             to,
		PollingIntervalSeconds: interval,
		IsActive:               true,
	}
}

func runningOf(pairs ...repository.CurrencyPair) map[int64]repository.CurrencyPair {
	m := make(map[int64]repository.CurrencyPair, len(pairs))
	for _, p := range pairs {
		m[p.ID] = p
	}
	return m
}

func TestPlanReconcile(t *testing.T) {
	rsdEur := pair(1, "RSD", "EUR", 3600)
	rsdUsd := pair(2, "RSD", "USD", 3600)
	eurUsd := pair(3, "EUR", "USD", 3600)
	gbpUsd := pair(4, "GBP", "USD", 3600)

	tests := []struct {
		name        string
		running     map[int64]repository.CurrencyPair
		desired     []repository.CurrencyPair
		wantStart   []repository.CurrencyPair
		wantStop    []int64
		wantRestart []repository.CurrencyPair
	}{
		{
			name:      "empty to three pairs",
			running:   runningOf(),
			desired:   []repository.CurrencyPair{eurUsd, rsdEur, rsdUsd},
			wantStart: []repository.CurrencyPair{rsdEur, rsdUsd, eurUsd},
		},
		{
			name:    "unchanged",
			running: runningOf(rsdEur, rsdUsd, eurUsd),
			desired: []repository.CurrencyPair{rsdEur, rsdUsd, eurUsd},
		},
		{
			name:      "one added",
			running:   runningOf(rsdEur, rsdUsd, eurUsd),
			desired:   []repository.CurrencyPair{rsdEur, rsdUsd, eurUsd, gbpUsd},
			wantStart: []repository.CurrencyPair{gbpUsd},
		},
		{
			name:     "one removed",
			running:  runningOf(rsdEur, rsdUsd, eurUsd),
			desired:  []repository.CurrencyPair{rsdEur, eurUsd},
			wantStop: []int64{2},
		},
		{
			name:        "interval changed",
			running:     runningOf(rsdEur, rsdUsd, eurUsd),
			desired:     []repository.CurrencyPair{rsdEur, pair(2, "RSD", "USD", 600), eurUsd},
			wantRestart: []repository.CurrencyPair{pair(2, "RSD", "USD", 600)},
		},
		{
			name:        "codes changed",
			running:     runningOf(rsdEur, rsdUsd, eurUsd),
			desired:     []repository.CurrencyPair{rsdEur, rsdUsd, pair(3, "CHF", "USD", 3600)},
			wantRestart: []repository.CurrencyPair{pair(3, "CHF", "USD", 3600)},
		},
		{
			name:     "all removed",
			running:  runningOf(eurUsd, rsdEur, rsdUsd),
			desired:  nil,
			wantStop: []int64{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, stop, restart := planReconcile(tt.running, tt.desired)
			if !reflect.DeepEqual(start, tt.wantStart) {
				t.Errorf("start = %v, want %v", start, tt.wantStart)
			}
			if !reflect.DeepEqual(stop, tt.wantStop) {
				t.Errorf("stop = %v, want %v", stop, tt.wantStop)
			}
			if !reflect.DeepEqual(restart, tt.wantRestart) {
				t.Errorf("restart = %v, want %v", restart, tt.wantRestart)
			}
		})
	}
}

func providerRow(mapping string) repository.Provider {
	return repository.Provider{
		ID:                  1,
		Name:                "fawazahmed0",
		AdapterType:         "generic_json",
		BaseUrl:             "https://example.test/{from}.min.json",
		RateJsonPath:        pgtype.Text{String: "{from}.{to}", Valid: true},
		CurrencyCodeMapping: []byte(mapping),
		IsActive:            true,
	}
}

func TestProvidersFingerprint(t *testing.T) {
	base := []repository.Provider{providerRow(`{"RSD":"rsd","EUR":"eur","USD":"usd"}`)}
	same := []repository.Provider{providerRow(`{"RSD":"rsd","EUR":"eur","USD":"usd"}`)}
	changed := []repository.Provider{providerRow(`{"RSD":"rsd","EUR":"eur","USD":"usd","GBP":"gbp"}`)}

	if providersFingerprint(base) != providersFingerprint(same) {
		t.Error("fingerprint differs for identical rows")
	}
	if providersFingerprint(base) == providersFingerprint(changed) {
		t.Error("fingerprint unchanged after currency_code_mapping change")
	}
}
