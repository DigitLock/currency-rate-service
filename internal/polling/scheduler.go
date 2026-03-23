package polling

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/DigitLock/currency-rate-service/internal/adapter"
	"github.com/DigitLock/currency-rate-service/internal/repository"
)

// Scheduler manages polling cycles for all active currency pairs.
type Scheduler struct {
	pool     *pgxpool.Pool
	queries  *repository.Queries
	registry *adapter.Registry
	logger   *slog.Logger
	wg       sync.WaitGroup
	cancel   context.CancelFunc
}

// NewScheduler creates a new polling scheduler.
func NewScheduler(pool *pgxpool.Pool, registry *adapter.Registry, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		pool:     pool,
		queries:  repository.New(pool),
		registry: registry,
		logger:   logger,
	}
}

// Start launches polling goroutines for all active pairs.
func (s *Scheduler) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	pairs, err := s.queries.GetActivePairs(ctx)
	if err != nil {
		return fmt.Errorf("load active pairs: %w", err)
	}

	if len(pairs) == 0 {
		s.logger.Warn("no active currency pairs found")
		return nil
	}

	for _, pair := range pairs {
		s.wg.Add(1)
		go s.pollPair(ctx, pair)
	}

	s.logger.Info("polling engine started", "pairs", len(pairs))
	return nil
}

// Stop signals all polling goroutines to stop and waits for completion.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.logger.Info("polling engine stopped")
}

// pollPair runs the polling loop for a single currency pair.
func (s *Scheduler) pollPair(ctx context.Context, pair repository.CurrencyPair) {
	defer s.wg.Done()

	pairLabel := fmt.Sprintf("%s→%s", pair.FromCurrency, pair.ToCurrency)
	interval := time.Duration(pair.PollingIntervalSeconds) * time.Second
	logger := s.logger.With("pair", pairLabel, "component", "polling")

	// Initial poll immediately
	s.executePollCycle(ctx, pair, logger)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("polling stopped")
			return
		case <-ticker.C:
			s.executePollCycle(ctx, pair, logger)
		}
	}
}

// executePollCycle performs a single polling cycle: fetch from primary, failover to backup if needed.
func (s *Scheduler) executePollCycle(ctx context.Context, pair repository.CurrencyPair, logger *slog.Logger) {
	start := time.Now()

	// Load provider assignments for this pair (fresh from DB each cycle — SRS 3.1.2)
	configs, err := s.queries.GetProvidersForPair(ctx, pair.ID)
	if err != nil {
		logger.Error("failed to load providers", "error", err)
		return
	}

	if len(configs) == 0 {
		logger.Error("no providers configured")
		return
	}

	adapterPair := adapter.CurrencyPair{
		ID:           pair.ID,
		FromCurrency: pair.FromCurrency,
		ToCurrency:   pair.ToCurrency,
	}

	// Try providers in priority order (primary first, then backup)
	for _, cfg := range configs {
		provider, err := s.registry.Get(cfg.ProviderName)
		if err != nil {
			logger.Warn("adapter not found", "provider", cfg.ProviderName, "error", err)
			continue
		}

		result, err := provider.FetchRate(ctx, adapterPair)
		if err != nil {
			logger.Warn("provider fetch failed",
				"provider", cfg.ProviderName,
				"priority", cfg.Priority,
				"error", err,
			)
			// Record failure
			if healthErr := s.queries.RecordFailure(ctx, repository.RecordFailureParams{
				ProviderID:       cfg.ProviderID,
				CurrencyPairID:   pair.ID,
				LastErrorMessage: pgTextFromString(err.Error()),
			}); healthErr != nil {
				logger.Error("failed to record health failure", "error", healthErr)
			}
			continue
		}

		// Success — store rate
		if _, err := s.queries.InsertRate(ctx, repository.InsertRateParams{
			CurrencyPairID:   pair.ID,
			SourceProviderID: cfg.ProviderID,
			Rate:             pgNumericFromFloat(result.Rate),
			IsOutdated:       false,
			FetchedAt:        pgTimestamptz(result.FetchedAt),
		}); err != nil {
			logger.Error("failed to store rate", "error", err)
			return
		}

		// Record success
		if err := s.queries.RecordSuccess(ctx, repository.RecordSuccessParams{
			ProviderID:     cfg.ProviderID,
			CurrencyPairID: pair.ID,
		}); err != nil {
			logger.Error("failed to record health success", "error", err)
		}

		logger.Info("rate fetched",
			"provider", cfg.ProviderName,
			"rate", result.Rate,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return
	}

	// All providers failed — mark outdated (SRS 2.3.4)
	if err := s.queries.MarkOutdated(ctx, pair.ID); err != nil {
		logger.Error("failed to mark rate as outdated", "error", err)
	}
	logger.Error("all providers failed, rate marked as outdated",
		"duration_ms", time.Since(start).Milliseconds(),
	)
}
