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

// Scheduler manages polling cycles for all active currency pairs and reloads
// business configuration from the database every reload interval (SRS 3.1.2).
type Scheduler struct {
	pool            *pgxpool.Pool
	queries         *repository.Queries
	registry        *adapter.Registry
	logger          *slog.Logger
	providerTimeout time.Duration
	reloadInterval  time.Duration
	wg              sync.WaitGroup
	cancel          context.CancelFunc

	// Reload state, accessed only from the reload path and Stop.
	mu                  sync.Mutex
	running             map[int64]runningPair
	providerFingerprint string
}

// runningPair is a pair with an active polling loop.
type runningPair struct {
	pair   repository.CurrencyPair
	cancel context.CancelFunc
}

// NewScheduler creates a new polling scheduler with its own adapter registry.
func NewScheduler(pool *pgxpool.Pool, logger *slog.Logger, providerTimeout, reloadInterval time.Duration) *Scheduler {
	return &Scheduler{
		pool:            pool,
		queries:         repository.New(pool),
		registry:        adapter.NewRegistry(),
		logger:          logger,
		providerTimeout: providerTimeout,
		reloadInterval:  reloadInterval,
		running:         make(map[int64]runningPair),
	}
}

// Start loads configuration once, launches polling for all active pairs and
// starts the configuration reload loop. A failed first load is returned as an error.
func (s *Scheduler) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	if err := s.reload(ctx); err != nil {
		s.cancel()
		return err
	}

	s.mu.Lock()
	pairs := len(s.running)
	s.mu.Unlock()

	if pairs == 0 {
		s.logger.Warn("no active currency pairs found, waiting for configuration reload")
	}

	s.wg.Add(1)
	go s.reloadLoop(ctx)

	s.logger.Info("polling engine started", "pairs", pairs, "reload_interval", s.reloadInterval.String())
	return nil
}

// Stop signals the reload loop and all polling goroutines to stop and waits for completion.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()

	s.mu.Lock()
	clear(s.running)
	s.mu.Unlock()

	s.logger.Info("polling engine stopped")
}

// reloadLoop reloads configuration once per reload interval until ctx is done.
func (s *Scheduler) reloadLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.reloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.reload(ctx); err != nil && ctx.Err() == nil {
				s.logger.Warn("configuration read failed, using previous configuration",
					"component", "config",
					"error", err,
				)
			}
		}
	}
}

// reload reads active providers and pairs, rebuilds the adapter registry if providers
// changed and reconciles polling loops with active pairs (SRS 3.1.2).
// If a read fails, nothing is changed and the error is returned.
func (s *Scheduler) reload(ctx context.Context) error {
	providers, err := s.queries.GetActiveProviders(ctx)
	if err != nil {
		return fmt.Errorf("load active providers: %w", err)
	}

	pairs, err := s.queries.GetActivePairs(ctx)
	if err != nil {
		return fmt.Errorf("load active pairs: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	providersChanged := false
	if fp := providersFingerprint(providers); fp != s.providerFingerprint {
		s.registry.ReplaceAll(buildProviders(providers, s.providerTimeout, s.logger))
		s.providerFingerprint = fp
		providersChanged = true
	}

	running := make(map[int64]repository.CurrencyPair, len(s.running))
	for id, rp := range s.running {
		running[id] = rp.pair
	}

	start, stop, restart := planReconcile(running, pairs)

	for _, id := range stop {
		s.running[id].cancel()
		delete(s.running, id)
	}
	for _, pair := range restart {
		s.running[pair.ID].cancel()
		s.startPair(ctx, pair)
	}
	for _, pair := range start {
		s.startPair(ctx, pair)
	}

	logArgs := []any{
		"component", "config",
		"started", len(start),
		"stopped", len(stop),
		"restarted", len(restart),
		"providers_changed", providersChanged,
	}
	if providersChanged || len(start) > 0 || len(stop) > 0 || len(restart) > 0 {
		s.logger.Info("configuration reloaded", logArgs...)
	} else {
		s.logger.Debug("configuration reloaded", logArgs...)
	}

	return nil
}

// startPair launches a polling loop for a pair with its own child context. Caller holds s.mu.
func (s *Scheduler) startPair(ctx context.Context, pair repository.CurrencyPair) {
	pairCtx, cancel := context.WithCancel(ctx)
	s.running[pair.ID] = runningPair{pair: pair, cancel: cancel}

	s.wg.Add(1)
	go s.pollPair(pairCtx, pair)
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
			// Pair loop or service is stopping — not a provider failure.
			if ctx.Err() != nil {
				return
			}
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
	if ctx.Err() != nil {
		return
	}
	if err := s.queries.MarkOutdated(ctx, pair.ID); err != nil {
		logger.Error("failed to mark rate as outdated", "error", err)
	}
	logger.Error("all providers failed, rate marked as outdated",
		"duration_ms", time.Since(start).Milliseconds(),
	)
}
