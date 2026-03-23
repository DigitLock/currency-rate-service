package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/DigitLock/currency-rate-service/internal/repository"
)

// Handler serves HTTP health check endpoints.
type Handler struct {
	pool      *pgxpool.Pool
	queries   *repository.Queries
	logger    *slog.Logger
	startedAt time.Time
}

// NewHandler creates a new health handler.
func NewHandler(pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{
		pool:      pool,
		queries:   repository.New(pool),
		logger:    logger,
		startedAt: time.Now(),
	}
}

// Healthz is the liveness probe — 200 if the process is running (SRS 3.2.3).
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Readyz is the readiness probe — 200 if DB is healthy and rates exist (SRS 3.2.3).
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check database connection
	dbStatus := "connected"
	if err := h.pool.Ping(ctx); err != nil {
		dbStatus = "disconnected"
	}

	// Count active pairs
	pairs, err := h.queries.GetActivePairs(ctx)
	activePairs := 0
	if err == nil {
		activePairs = len(pairs)
	}

	// Count outdated pairs
	outdatedPairs := 0
	for _, pair := range pairs {
		rate, err := h.queries.GetLatestRate(ctx, pair.ID)
		if err == nil && rate.IsOutdated {
			outdatedPairs++
		}
	}

	// Provider health
	allHealth, _ := h.queries.GetAllHealth(ctx)
	var providers []providerStatus
	for _, ph := range allHealth {
		status := "healthy"
		if ph.ConsecutiveFailures > 0 {
			status = "degraded"
		}
		if ph.ConsecutiveFailures > 3 {
			status = "unhealthy"
		}

		ps := providerStatus{
			Name:                ph.ProviderName,
			Status:              status,
			ConsecutiveFailures: int(ph.ConsecutiveFailures),
		}
		if ph.LastSuccessAt.Valid {
			t := ph.LastSuccessAt.Time.Format(time.RFC3339)
			ps.LastSuccessAt = &t
		}
		providers = append(providers, ps)
	}

	// Overall status
	overallStatus := "healthy"
	if dbStatus != "connected" {
		overallStatus = "unhealthy"
	} else if outdatedPairs > 0 {
		overallStatus = "degraded"
	}

	resp := readyzResponse{
		Status:        overallStatus,
		Database:      dbStatus,
		ActivePairs:   activePairs,
		OutdatedPairs: outdatedPairs,
		Providers:     providers,
		UptimeSeconds: int(time.Since(h.startedAt).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	if overallStatus == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	json.NewEncoder(w).Encode(resp)
}

type readyzResponse struct {
	Status        string           `json:"status"`
	Database      string           `json:"database"`
	ActivePairs   int              `json:"active_pairs"`
	OutdatedPairs int              `json:"outdated_pairs"`
	Providers     []providerStatus `json:"providers"`
	UptimeSeconds int              `json:"uptime_seconds"`
}

type providerStatus struct {
	Name                string  `json:"name"`
	Status              string  `json:"status"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	LastSuccessAt       *string `json:"last_success_at,omitempty"`
}
