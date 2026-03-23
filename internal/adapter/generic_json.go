package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GenericJSONConfig holds the database-driven configuration for a generic JSON provider.
type GenericJSONConfig struct {
	ProviderName        string
	BaseURL             string            // URL template with {from} and {to} placeholders
	RateJSONPath        string            // Dot-notation path with {from} and {to} placeholders
	CurrencyCodeMapping map[string]string // Internal code → provider code (e.g., "RSD" → "rsd")
}

// GenericJSONAdapter implements RateProvider for standard REST JSON APIs (SRS 5.2).
type GenericJSONAdapter struct {
	config     GenericJSONConfig
	httpClient *http.Client
}

// NewGenericJSONAdapter creates a new adapter with the given config and HTTP timeout.
func NewGenericJSONAdapter(cfg GenericJSONConfig, timeout time.Duration) *GenericJSONAdapter {
	return &GenericJSONAdapter{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the provider's unique identifier.
func (a *GenericJSONAdapter) Name() string {
	return a.config.ProviderName
}

// FetchRate retrieves the current exchange rate from the external provider (SRS 5.2 algorithm).
func (a *GenericJSONAdapter) FetchRate(ctx context.Context, pair CurrencyPair) (RateResult, error) {
	// Step 1: Resolve provider-specific currency codes
	fromCode := a.mapCurrencyCode(pair.FromCurrency)
	toCode := a.mapCurrencyCode(pair.ToCurrency)

	// Step 2: Build request URL
	url := a.buildURL(fromCode, toCode)

	// Step 3: Execute HTTP GET
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return RateResult{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return RateResult{}, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return RateResult{}, fmt.Errorf("HTTP GET %s: status %d", url, resp.StatusCode)
	}

	// Step 4: Parse JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RateResult{}, fmt.Errorf("read response body: %w", err)
	}

	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return RateResult{}, fmt.Errorf("parse JSON: %w", err)
	}

	// Step 5: Navigate to rate value using JSONPath
	path := a.buildJSONPath(fromCode, toCode)
	rate, err := navigateJSON(data, path)
	if err != nil {
		return RateResult{}, fmt.Errorf("extract rate at path %q: %w", path, err)
	}

	// Step 6: Validate rate
	if rate <= 0 {
		return RateResult{}, fmt.Errorf("invalid rate value: %f", rate)
	}

	// Step 7: Return result
	return RateResult{
		Rate:       rate,
		FetchedAt:  time.Now().UTC(),
		SourceName: a.config.ProviderName,
	}, nil
}

// mapCurrencyCode resolves provider-specific currency code via mapping.
func (a *GenericJSONAdapter) mapCurrencyCode(code string) string {
	if a.config.CurrencyCodeMapping == nil {
		return code
	}
	if mapped, ok := a.config.CurrencyCodeMapping[code]; ok {
		return mapped
	}
	return code
}

// buildURL substitutes {from} and {to} placeholders in the URL template.
func (a *GenericJSONAdapter) buildURL(from, to string) string {
	url := strings.ReplaceAll(a.config.BaseURL, "{from}", from)
	url = strings.ReplaceAll(url, "{to}", to)
	return url
}

// buildJSONPath substitutes {from} and {to} placeholders in the JSONPath template.
func (a *GenericJSONAdapter) buildJSONPath(from, to string) string {
	path := strings.ReplaceAll(a.config.RateJSONPath, "{from}", from)
	path = strings.ReplaceAll(path, "{to}", to)
	return path
}

// navigateJSON traverses a parsed JSON structure using dot-notation path (e.g., "rsd.eur" or "rates.EUR").
func navigateJSON(data any, path string) (float64, error) {
	parts := strings.Split(path, ".")
	current := data

	for _, key := range parts {
		obj, ok := current.(map[string]any)
		if !ok {
			return 0, fmt.Errorf("expected object at key %q, got %T", key, current)
		}
		current, ok = obj[key]
		if !ok {
			return 0, fmt.Errorf("key %q not found", key)
		}
	}

	return toFloat64(current)
}

// toFloat64 converts a JSON value to float64.
func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case json.Number:
		return val.Float64()
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}
