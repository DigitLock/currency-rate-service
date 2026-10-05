package grpc

import (
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/DigitLock/currency-rate-service/internal/repository"
)

func TestRateToProto(t *testing.T) {
	fetchedAt := time.Date(2026, 10, 5, 16, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		from, to    string
		numeric     pgtype.Numeric
		rateText    string
		isOutdated  bool
		provider    string
		wantRate    float64
		wantDecimal string
	}{
		{
			name:        "GBP to USD",
			from:        "GBP",
			to:          "USD",
			numeric:     pgtype.Numeric{Int: big.NewInt(13225000000), Exp: -10, Valid: true},
			rateText:    "1.3225000000",
			provider:    "frankfurter",
			wantRate:    1.3225,
			wantDecimal: "1.3225000000",
		},
		{
			name:        "RSD to EUR, outdated",
			from:        "RSD",
			to:          "EUR",
			numeric:     pgtype.Numeric{Int: big.NewInt(85300000), Exp: -10, Valid: true},
			rateText:    "0.0085300000",
			isOutdated:  true,
			provider:    "fawazahmed0",
			wantRate:    0.00853,
			wantDecimal: "0.0085300000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := repository.GetLatestRateRow{
				Rate:               tt.numeric,
				IsOutdated:         tt.isOutdated,
				FetchedAt:          pgtype.Timestamptz{Time: fetchedAt, Valid: true},
				SourceProviderName: tt.provider,
				RateText:           tt.rateText,
			}

			got := rateToProto(tt.from, tt.to, row)

			if got.RateDecimal != tt.wantDecimal {
				t.Errorf("RateDecimal = %q, want %q", got.RateDecimal, tt.wantDecimal)
			}
			if got.Rate != tt.wantRate {
				t.Errorf("Rate = %v, want %v", got.Rate, tt.wantRate)
			}
			if got.FromCurrency != tt.from || got.ToCurrency != tt.to {
				t.Errorf("currencies = %s→%s, want %s→%s", got.FromCurrency, got.ToCurrency, tt.from, tt.to)
			}
			if !got.UpdatedAt.AsTime().Equal(fetchedAt) {
				t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt.AsTime(), fetchedAt)
			}
			if got.IsOutdated != tt.isOutdated {
				t.Errorf("IsOutdated = %v, want %v", got.IsOutdated, tt.isOutdated)
			}
			if got.SourceProvider != tt.provider {
				t.Errorf("SourceProvider = %q, want %q", got.SourceProvider, tt.provider)
			}
		})
	}
}
