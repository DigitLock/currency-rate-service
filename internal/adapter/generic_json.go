package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
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

	// Step 4: Parse JSON response, keeping numbers as their JSON text (no float conversion)
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()

	var data any
	if err := dec.Decode(&data); err != nil {
		return RateResult{}, fmt.Errorf("parse JSON: %w", err)
	}

	// Step 5: Navigate to rate value using JSONPath
	path := a.buildJSONPath(fromCode, toCode)
	rate, err := navigateJSON(data, path)
	if err != nil {
		return RateResult{}, fmt.Errorf("extract rate at path %q: %w", path, err)
	}

	// Step 6: Validate rate — must be a positive decimal number
	if !isPositiveDecimal(rate) {
		return RateResult{}, fmt.Errorf("invalid rate value: %s", rate)
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
func navigateJSON(data any, path string) (string, error) {
	parts := strings.Split(path, ".")
	current := data

	for _, key := range parts {
		obj, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("expected object at key %q, got %T", key, current)
		}
		current, ok = obj[key]
		if !ok {
			return "", fmt.Errorf("key %q not found", key)
		}
	}

	return toDecimal(current)
}

// jsonNumberRe matches the JSON number grammar.
var jsonNumberRe = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// maxDecimalExponent bounds exponents so a malformed response cannot force a huge expansion.
const maxDecimalExponent = 100

// toDecimal converts a JSON value to a plain decimal string without going through float64 (SRS 5.2).
// A literal without an exponent is returned unchanged; an exponent is expanded exactly.
func toDecimal(v any) (string, error) {
	var s string
	switch val := v.(type) {
	case json.Number:
		s = val.String()
	case string:
		s = val
	default:
		return "", fmt.Errorf("cannot convert %T to decimal", v)
	}

	if !jsonNumberRe.MatchString(s) {
		return "", fmt.Errorf("invalid decimal %q", s)
	}

	expIdx := strings.IndexAny(s, "eE")
	if expIdx < 0 {
		return s, nil
	}

	mantissa := s[:expIdx]
	exp, err := strconv.Atoi(s[expIdx+1:])
	if err != nil || exp > maxDecimalExponent || exp < -maxDecimalExponent {
		return "", fmt.Errorf("invalid decimal %q: exponent out of range", s)
	}

	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return "", fmt.Errorf("invalid decimal %q", s)
	}

	fracDigits := 0
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		fracDigits = len(mantissa) - dot - 1
	}

	// FloatString is exact here: the value has at most fracDigits−exp fractional digits.
	return r.FloatString(max(0, fracDigits-exp)), nil
}

// isPositiveDecimal reports whether s parses as a decimal number greater than zero.
func isPositiveDecimal(s string) bool {
	r, ok := new(big.Rat).SetString(s)
	return ok && r.Sign() > 0
}
