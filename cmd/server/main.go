package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/DigitLock/currency-rate-service/internal/config"
	grpcserver "github.com/DigitLock/currency-rate-service/internal/grpc"
	"github.com/DigitLock/currency-rate-service/internal/grpc/pb"
	"github.com/DigitLock/currency-rate-service/internal/health"
	"github.com/DigitLock/currency-rate-service/internal/polling"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	slog.Info("starting currency-rate-service",
		"grpc_port", cfg.GRPCPort,
		"health_port", cfg.HealthHTTPPort,
		"log_level", cfg.LogLevel,
		"config_reload_interval", cfg.ConfigReloadInterval.String(),
	)

	// Connect to database
	ctx := context.Background()
	pool, err := connectDB(ctx, cfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("database connected",
		"max_conns", cfg.DBPoolMaxConns,
		"min_conns", cfg.DBPoolMinConns,
	)

	// Start polling engine (loads providers and pairs, then reloads them every CONFIG_RELOAD_INTERVAL — SRS 3.1.2)
	scheduler := polling.NewScheduler(pool, logger, cfg.ProviderHTTPTimeout, cfg.ConfigReloadInterval)
	if err := scheduler.Start(ctx); err != nil {
		slog.Error("failed to start polling engine", "error", err)
		os.Exit(1)
	}

	// Start gRPC server
	grpcSrv, grpcLis, err := startGRPCServer(cfg, pool, logger)
	if err != nil {
		slog.Error("failed to start gRPC server", "error", err)
		os.Exit(1)
	}

	// Start health HTTP server
	healthHandler := health.NewHandler(pool, logger)
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", healthHandler.Healthz)
	healthMux.HandleFunc("/readyz", healthHandler.Readyz)

	healthSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HealthHTTPPort),
		Handler: healthMux,
	}

	go func() {
		slog.Info("health HTTP server listening", "port", cfg.HealthHTTPPort)
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health HTTP server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	// Graceful shutdown
	grpcSrv.GracefulStop()
	slog.Info("gRPC server stopped", "port", cfg.GRPCPort)

	healthSrv.Shutdown(context.Background())
	slog.Info("health HTTP server stopped", "port", cfg.HealthHTTPPort)

	scheduler.Stop()

	_ = grpcLis

	slog.Info("currency-rate-service stopped")
}

func startGRPCServer(cfg *config.Config, pool *pgxpool.Pool, logger *slog.Logger) (*grpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return nil, nil, fmt.Errorf("listen on port %d: %w", cfg.GRPCPort, err)
	}

	srv := grpc.NewServer()
	pb.RegisterCurrencyRateServiceServer(srv, grpcserver.NewServer(pool, logger))
	reflection.Register(srv)

	go func() {
		slog.Info("gRPC server listening", "port", cfg.GRPCPort)
		if err := srv.Serve(lis); err != nil {
			slog.Error("gRPC server error", "error", err)
		}
	}()

	return srv, lis, nil
}

func setupLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func connectDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.DBPoolMaxConns)
	poolCfg.MinConns = int32(cfg.DBPoolMinConns)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
