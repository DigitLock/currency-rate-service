package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	GRPCPort            int
	HealthHTTPPort      int
	DatabaseURL         string
	DBPoolMaxConns      int
	DBPoolMinConns      int
	LogLevel            string
	LogFormat           string
	ProviderHTTPTimeout time.Duration
	ShutdownTimeout     time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:            envInt("GRPC_PORT", 50052),
		HealthHTTPPort:      envInt("HEALTH_HTTP_PORT", 8090),
		DatabaseURL:         envString("DATABASE_URL", ""),
		DBPoolMaxConns:      envInt("DB_POOL_MAX_CONNS", 10),
		DBPoolMinConns:      envInt("DB_POOL_MIN_CONNS", 2),
		LogLevel:            envString("LOG_LEVEL", "info"),
		LogFormat:           envString("LOG_FORMAT", "json"),
		ProviderHTTPTimeout: envDuration("PROVIDER_HTTP_TIMEOUT", 10*time.Second),
		ShutdownTimeout:     envDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return nil, ErrDatabaseURLRequired
	}

	return cfg, nil
}

var ErrDatabaseURLRequired = errorf("DATABASE_URL is required")

func envString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

type configError string

func errorf(msg string) configError { return configError(msg) }
func (e configError) Error() string { return string(e) }
