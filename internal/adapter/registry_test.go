package adapter

import (
	"context"
	"testing"
)

type stubProvider struct{ name string }

func (s stubProvider) FetchRate(context.Context, CurrencyPair) (RateResult, error) {
	return RateResult{}, nil
}

func (s stubProvider) Name() string { return s.name }

func TestRegistryReplaceAll(t *testing.T) {
	r := NewRegistry()
	r.Register(stubProvider{name: "old"})
	r.Register(stubProvider{name: "kept"})

	r.ReplaceAll([]RateProvider{stubProvider{name: "kept"}, stubProvider{name: "new"}})

	for _, name := range []string{"kept", "new"} {
		if _, err := r.Get(name); err != nil {
			t.Errorf("Get(%q) after ReplaceAll: %v", name, err)
		}
	}
	if _, err := r.Get("old"); err == nil {
		t.Error("Get(\"old\") after ReplaceAll: expected error, got nil")
	}
}
