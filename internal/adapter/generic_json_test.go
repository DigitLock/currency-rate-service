package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenericJSONAdapterFetchRate(t *testing.T) {
	gbpUsd := CurrencyPair{ID: 4, FromCurrency: "GBP", ToCurrency: "USD"}
	fawazMapping := map[string]string{"GBP": "gbp", "USD": "usd"}

	tests := []struct {
		name    string
		body    string
		path    string
		mapping map[string]string
		want    string
		wantErr bool
	}{
		{
			name: "frankfurter shape",
			body: `{"amount":1.0,"base":"GBP","date":"2026-10-02","rates":{"USD":1.3225}}`,
			path: "rates.{to}",
			want: "1.3225",
		},
		{
			name:    "fawazahmed0 shape with mapping",
			body:    `{"date":"2026-10-04","gbp":{"usd":1.32416319}}`,
			path:    "{from}.{to}",
			mapping: fawazMapping,
			want:    "1.32416319",
		},
		{
			name: "digits beyond float64 precision preserved",
			body: `{"rates":{"USD":0.123456789012345678}}`,
			path: "rates.{to}",
			want: "0.123456789012345678",
		},
		{
			name: "negative exponent",
			body: `{"rates":{"USD":1.5e-5}}`,
			path: "rates.{to}",
			want: "0.000015",
		},
		{
			name: "positive exponent",
			body: `{"rates":{"USD":2.5E+3}}`,
			path: "rates.{to}",
			want: "2500",
		},
		{
			name: "string value",
			body: `{"rates":{"USD":"1.25"}}`,
			path: "rates.{to}",
			want: "1.25",
		},
		{name: "zero", body: `{"rates":{"USD":0}}`, path: "rates.{to}", wantErr: true},
		{name: "negative", body: `{"rates":{"USD":-1.2}}`, path: "rates.{to}", wantErr: true},
		{name: "boolean", body: `{"rates":{"USD":true}}`, path: "rates.{to}", wantErr: true},
		{name: "object", body: `{"rates":{"USD":{"x":1}}}`, path: "rates.{to}", wantErr: true},
		{name: "non-numeric string", body: `{"rates":{"USD":"abc"}}`, path: "rates.{to}", wantErr: true},
		{name: "missing key", body: `{"rates":{"EUR":1.1}}`, path: "rates.{to}", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			a := NewGenericJSONAdapter(GenericJSONConfig{
				ProviderName:        "test",
				BaseURL:             srv.URL + "/{from}",
				RateJSONPath:        tt.path,
				CurrencyCodeMapping: tt.mapping,
			}, 5*time.Second)

			got, err := a.FetchRate(context.Background(), gbpUsd)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("FetchRate() = %q, want error", got.Rate)
				}
				return
			}
			if err != nil {
				t.Fatalf("FetchRate() error: %v", err)
			}
			if got.Rate != tt.want {
				t.Errorf("Rate = %q, want %q", got.Rate, tt.want)
			}
		})
	}
}

func TestToDecimalExponent(t *testing.T) {
	tests := map[string]string{
		"1.5e-5": "0.000015",
		"2.5E+3": "2500",
		"1e-10":  "0.0000000001",
	}
	for in, want := range tests {
		got, err := toDecimal(json.Number(in))
		if err != nil {
			t.Errorf("toDecimal(%s) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("toDecimal(%s) = %q, want %q", in, got, want)
		}
	}
}
