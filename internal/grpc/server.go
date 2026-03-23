package grpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/DigitLock/currency-rate-service/internal/grpc/pb"
	"github.com/DigitLock/currency-rate-service/internal/repository"
)

// Server implements the CurrencyRateService gRPC API.
type Server struct {
	pb.UnimplementedCurrencyRateServiceServer
	queries *repository.Queries
	logger  *slog.Logger
}

// NewServer creates a new gRPC server.
func NewServer(pool *pgxpool.Pool, logger *slog.Logger) *Server {
	return &Server{
		queries: repository.New(pool),
		logger:  logger,
	}
}

// GetRate returns the current exchange rate for a single currency pair (SRS 2.1.2.1).
func (s *Server) GetRate(ctx context.Context, req *pb.GetRateRequest) (*pb.GetRateResponse, error) {
	if req.FromCurrency == "" || req.ToCurrency == "" {
		return nil, status.Error(codes.InvalidArgument, "from_currency and to_currency are required")
	}

	if !isValidCurrencyCode(req.FromCurrency) || !isValidCurrencyCode(req.ToCurrency) {
		return nil, status.Error(codes.InvalidArgument, "invalid currency code format")
	}

	// Find the pair
	pair, err := s.queries.FindPairByCode(ctx, repository.FindPairByCodeParams{
		FromCurrency: req.FromCurrency,
		ToCurrency:   req.ToCurrency,
	})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "currency pair %s→%s not configured", req.FromCurrency, req.ToCurrency)
	}

	// Get latest rate
	rate, err := s.queries.GetLatestRate(ctx, pair.ID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no rate data for %s→%s", req.FromCurrency, req.ToCurrency)
	}

	return &pb.GetRateResponse{
		Rate: rateToProto(req.FromCurrency, req.ToCurrency, rate),
	}, nil
}

// GetRates returns current rates for multiple target currencies (SRS 2.1.2.2).
func (s *Server) GetRates(ctx context.Context, req *pb.GetRatesRequest) (*pb.GetRatesResponse, error) {
	if req.BaseCurrency == "" {
		return nil, status.Error(codes.InvalidArgument, "base_currency is required")
	}
	if len(req.TargetCurrencies) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one target_currency is required")
	}
	if !isValidCurrencyCode(req.BaseCurrency) {
		return nil, status.Error(codes.InvalidArgument, "invalid base_currency format")
	}

	var rates []*pb.Rate

	for _, target := range req.TargetCurrencies {
		if !isValidCurrencyCode(target) {
			continue // Skip invalid, don't fail the batch
		}

		pair, err := s.queries.FindPairByCode(ctx, repository.FindPairByCodeParams{
			FromCurrency: req.BaseCurrency,
			ToCurrency:   target,
		})
		if err != nil {
			continue // Pair not configured — omit from response (SRS 2.1.2.2)
		}

		rate, err := s.queries.GetLatestRate(ctx, pair.ID)
		if err != nil {
			continue // No rate yet — omit
		}

		rates = append(rates, rateToProto(req.BaseCurrency, target, rate))
	}

	return &pb.GetRatesResponse{Rates: rates}, nil
}

// ListSupportedPairs returns all configured currency pairs (SRS 2.1.2.3).
func (s *Server) ListSupportedPairs(ctx context.Context, _ *pb.ListSupportedPairsRequest) (*pb.ListSupportedPairsResponse, error) {
	pairs, err := s.queries.GetAllPairs(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load currency pairs")
	}

	var result []*pb.CurrencyPair
	for _, p := range pairs {
		result = append(result, &pb.CurrencyPair{
			FromCurrency:           p.FromCurrency,
			ToCurrency:             p.ToCurrency,
			PollingIntervalSeconds: p.PollingIntervalSeconds,
			IsActive:               p.IsActive,
		})
	}

	return &pb.ListSupportedPairsResponse{Pairs: result}, nil
}

// rateToProto converts a DB rate row to a proto Rate message.
func rateToProto(from, to string, row repository.GetLatestRateRow) *pb.Rate {
	return &pb.Rate{
		FromCurrency:   from,
		ToCurrency:     to,
		Rate:           numericToFloat64(row.Rate),
		UpdatedAt:      timestamppb.New(row.FetchedAt.Time),
		IsOutdated:     row.IsOutdated,
		SourceProvider: row.SourceProviderName,
	}
}

// numericToFloat64 converts pgtype.Numeric to float64.
func numericToFloat64(n pgtype.Numeric) float64 {
	f, _ := n.Float64Value()
	return f.Float64
}

// isValidCurrencyCode checks if the code matches ^[A-Z]{3,5}$.
func isValidCurrencyCode(code string) bool {
	if len(code) < 3 || len(code) > 5 {
		return false
	}
	for _, c := range code {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// formatPgTimestamptz formats a pgtype.Timestamptz for logging.
func formatPgTimestamptz(t pgtype.Timestamptz) string {
	if t.Valid {
		return t.Time.String()
	}
	return "null"
}

// Ensure interface compliance at compile time.
var _ pb.CurrencyRateServiceServer = (*Server)(nil)

// Suppress unused import warning.
var _ = fmt.Sprintf
