package adapter

import (
	"context"
	"time"
)

// CurrencyPair represents a currency pair for rate fetching.
type CurrencyPair struct {
	ID           int64
	FromCurrency string
	ToCurrency   string
}

// RateResult represents a successfully fetched exchange rate.
type RateResult struct {
	Rate       float64
	FetchedAt  time.Time
	SourceName string
}

// RateProvider is the contract every provider adapter must implement (SRS 5.1).
type RateProvider interface {
	// FetchRate retrieves the current exchange rate for a pair.
	// Must respect context deadline (PROVIDER_HTTP_TIMEOUT).
	FetchRate(ctx context.Context, pair CurrencyPair) (RateResult, error)

	// Name returns the provider's unique identifier, matching providers.name in the DB.
	Name() string
}
