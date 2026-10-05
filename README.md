# Currency Rate Service

Autonomous microservice that collects, stores, and serves live and historical currency exchange rates via gRPC. Part of the [Expense Tracker](https://github.com/DigitLock/expense-tracker) ecosystem.

## 🎯 Project Status

- ✅ **Business Requirements** — Complete (BRD v1.0)
- ✅ **System Requirements** — Complete (SRS v1.0)
- ✅ **Core Implementation** — Complete (Stages 1–6)
- 📋 **Testing & QA** — Planned
- 📋 **Docker & Deployment** — Planned

## ✨ Features

- 💱 **Multi-currency rates** (RSD→EUR, RSD→USD, EUR→USD, GBP→USD, CHF→USD)
- 🔄 **Automated polling** with configurable per-pair intervals
- 🛡️ **Provider failover** — primary/backup with automatic switchover
- ⚠️ **Staleness tracking** — last known rate always available, flagged when outdated
- 🗄️ **Database-driven config** — add pairs and providers at runtime; changes apply within `CONFIG_RELOAD_INTERVAL`, no restart
- 🔌 **Hybrid adapter architecture** — generic JSON (config-only) + custom interface (code)
- 📈 **Historical data** — every poll inserts a new record from day one
- 🏥 **Health monitoring** — per-provider health tracking, HTTP health endpoints
- 📡 **gRPC API** — unauthenticated, public rate data for any ecosystem consumer

## 🏗️ Tech Stack

### Backend
- **Language**: Go 1.25+
- **Database**: PostgreSQL (pgx/v5, sqlc)
- **API**: gRPC (Protocol Buffers)
- **Logging**: slog (structured JSON)
- **Proto Tooling**: protoc, Makefile

### Infrastructure
- **Deployment**: Docker (standalone container) — planned
- **Server**: Hetzner VPS (same host as Expense Tracker)
- **DNS**: Cloudflare (HTTP health endpoint only — gRPC via direct IP)

## 📡 gRPC API

**Service:** `currency_rate.v1.CurrencyRateService`
**Port:** `50052`

Proto file: `proto/currency_rate/v1/service.proto`

| Method | Description |
|--------|-------------|
| `GetRate` | Current rate for a single currency pair |
| `GetRates` | Current rates for multiple targets relative to a base currency |
| `ListSupportedPairs` | All configured pairs with polling metadata |

**Authentication**: None — exchange rates are public data.

**Rate precision**: every `Rate` returns both `rate` (double, kept for compatibility) and `rate_decimal` — the exact stored value as a decimal string with 10 fractional digits (e.g., `"1.3201000000"`). Use `rate_decimal` for exact arithmetic.

**Testing with grpcurl:**
```bash
# List supported pairs
grpcurl -plaintext localhost:50052 \
  currency_rate.v1.CurrencyRateService/ListSupportedPairs

# Get single rate
grpcurl -plaintext -d '{"from_currency":"RSD","to_currency":"EUR"}' \
  localhost:50052 \
  currency_rate.v1.CurrencyRateService/GetRate

# Get GBP→USD rate
grpcurl -plaintext -d '{"from_currency":"GBP","to_currency":"USD"}' \
  localhost:50052 \
  currency_rate.v1.CurrencyRateService/GetRate

# Get multiple rates
grpcurl -plaintext -d '{"base_currency":"RSD","target_currencies":["EUR","USD"]}' \
  localhost:50052 \
  currency_rate.v1.CurrencyRateService/GetRates
```

## 🗄️ Database Schema

Dedicated PostgreSQL database with 5 tables:

| Table | Purpose |
|-------|---------|
| `currency_pairs` | Configured pairs with polling intervals (business config) |
| `providers` | Rate provider definitions (URL template, JSONPath, adapter type) |
| `pair_provider_config` | Primary/backup provider assignments per pair |
| `rates` | Exchange rate records — current and historical (append-only) |
| `provider_health` | Per-provider, per-pair health tracking |

**ID type**: BIGSERIAL (auto-increment), consistent with Expense Tracker.

See the [SRS](Documentation/currency_rate_service_srs.md) Section 2.4 for full schema, constraints, and indexes.

## 🌐 External Rate Providers (v1)

| Provider | Pairs | Rate Limits | Auth |
|----------|-------|-------------|------|
| [fawazahmed0](https://github.com/fawazahmed0/exchange-api) (CDN) | RSD→EUR, RSD→USD, EUR→USD, GBP→USD, CHF→USD | None | None |
| [fawazahmed0](https://github.com/fawazahmed0/exchange-api) (pages.dev fallback) | RSD→EUR, RSD→USD, EUR→USD, GBP→USD, CHF→USD | None | None |
| [ExchangeRate-API](https://www.exchangerate-api.com/docs/free) (open access) | RSD→EUR, RSD→USD, EUR→USD, GBP→USD, CHF→USD | ~1/day recommended | None |
| [Frankfurter](https://frankfurter.dev/) (ECB data) | EUR→USD, GBP→USD, CHF→USD | None | None |

**Note**: Frankfurter does not support RSD (not published by the ECB).

## ⚙️ Configuration

### System (environment variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | — (required) | PostgreSQL connection string |
| `GRPC_PORT` | `50052` | gRPC server port |
| `HEALTH_HTTP_PORT` | `8090` | HTTP health endpoint port |
| `LOG_LEVEL` | `info` | Logging level: debug, info, warn, error |
| `LOG_FORMAT` | `json` | Log format: json or text |
| `PROVIDER_HTTP_TIMEOUT` | `10s` | HTTP timeout for provider requests |
| `SHUTDOWN_TIMEOUT` | `15s` | Graceful shutdown deadline |
| `DB_POOL_MAX_CONNS` | `10` | Maximum DB connection pool size |
| `DB_POOL_MIN_CONNS` | `2` | Minimum idle DB connections |
| `CONFIG_RELOAD_INTERVAL` | `60s` | How often pairs and providers are reloaded from the database |

### Business (database-driven)

Currency pairs, providers, polling intervals, and pair-provider assignments are managed in the database. Changes take effect within `CONFIG_RELOAD_INTERVAL` (default 60s) — no service restart required.

## 📚 Documentation

Located in `Documentation/`:

- [`currency_rate_service_brd.md`](Documentation/currency_rate_service_brd.md) — Business Requirements Document (12 BRs, stakeholders, risks, benefits)
- [`currency_rate_service_brd_amendment_1.md`](Documentation/currency_rate_service_brd_amendment_1.md) — BRD Amendment 1 (CAS consumer: GBP→USD and CHF→USD pairs, exact decimal rate, configuration hot reload)
- [`currency_rate_service_srs.md`](Documentation/currency_rate_service_srs.md) — System Requirements Specification (gRPC API contract, 4 use cases, 5-table data model, 6 ADRs, provider adapter spec)

## 📋 Project Structure

```
currency-rate-service/
├── cmd/
│   └── server/
│       └── main.go              # Application entrypoint
├── internal/
│   ├── adapter/
│   │   ├── adapter.go           # RateProvider interface, CurrencyPair, RateResult
│   │   ├── generic_json.go      # Generic JSON adapter (URL template + JSONPath)
│   │   └── registry.go          # Adapter registry (resolve by provider name)
│   ├── config/
│   │   └── config.go            # Environment variable loading with defaults
│   ├── grpc/
│   │   ├── pb/
│   │   │   ├── service.pb.go        # Generated protobuf types
│   │   │   └── service_grpc.pb.go   # Generated gRPC server/client stubs
│   │   └── server.go            # gRPC handlers (GetRate, GetRates, ListSupportedPairs)
│   ├── health/
│   │   └── handler.go           # HTTP health endpoints (/healthz, /readyz)
│   ├── polling/
│   │   ├── scheduler.go         # Polling engine (per-pair goroutines, failover)
│   │   ├── reload.go            # Config reload loop, pair reconciliation, provider registry rebuild
│   │   └── pgconv.go            # pgx type conversion helpers
│   └── repository/
│       ├── queries/                 # SQL query sources
│       │   ├── currency_pairs.sql
│       │   ├── providers.sql
│       │   ├── pair_provider_config.sql
│       │   ├── rates.sql
│       │   └── provider_health.sql
│       ├── models.go                # sqlc-generated models
│       ├── db.go                    # sqlc-generated DB interface
│       ├── currency_pairs.sql.go    # sqlc-generated query functions
│       ├── providers.sql.go
│       ├── pair_provider_config.sql.go
│       ├── rates.sql.go
│       └── provider_health.sql.go
├── migrations/                  # Versioned SQL (golang-migrate format)
│   ├── 001_currency_pairs.up.sql
│   ├── 001_currency_pairs.down.sql
│   ├── 002_providers.up.sql
│   ├── 002_providers.down.sql
│   ├── 003_pair_provider_config.up.sql
│   ├── 003_pair_provider_config.down.sql
│   ├── 004_rates.up.sql
│   ├── 004_rates.down.sql
│   ├── 005_provider_health.up.sql
│   ├── 005_provider_health.down.sql
│   ├── 006_pairs_gbp_chf.up.sql
│   └── 006_pairs_gbp_chf.down.sql
├── proto/
│   └── currency_rate/v1/
│       └── service.proto        # gRPC service definition
├── Documentation/
│   ├── currency_rate_service_brd.md
│   ├── currency_rate_service_brd_amendment_1.md
│   └── currency_rate_service_srs.md
├── .env.example
├── .gitignore
├── go.mod
├── go.sum
├── LICENSE
├── Makefile                     # Proto codegen targets
├── sqlc.yaml                    # sqlc configuration
└── README.md
```

## 🚀 Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL 15+
- protoc + protoc-gen-go + protoc-gen-go-grpc
- grpcurl (for testing)

### Development

```bash
# Clone
git clone https://github.com/DigitLock/currency-rate-service.git
cd currency-rate-service

# Create database and run migrations
createdb currency_rates_dev
psql -d currency_rates_dev -f migrations/001_currency_pairs.up.sql
psql -d currency_rates_dev -f migrations/002_providers.up.sql
psql -d currency_rates_dev -f migrations/003_pair_provider_config.up.sql
psql -d currency_rates_dev -f migrations/004_rates.up.sql
psql -d currency_rates_dev -f migrations/005_provider_health.up.sql
psql -d currency_rates_dev -f migrations/006_pairs_gbp_chf.up.sql

# Copy and edit env
cp .env.example .env
# Edit .env with your DATABASE_URL

# Generate proto (if changed)
make proto

# Run
export $(grep -v '^#' .env | xargs) && go run cmd/server/main.go
```

### Verification

```bash
# Health check
curl -s http://localhost:8090/healthz
curl -s http://localhost:8090/readyz | jq

# gRPC
grpcurl -plaintext localhost:50052 \
  currency_rate.v1.CurrencyRateService/ListSupportedPairs
```

## 🚀 Roadmap

### Phase 1: Documentation ✅
- [x] Business Requirements Document (BRD v1.0)
- [x] System Requirements Specification (SRS v1.0)
- [x] Architecture Decision Records (6 ADRs)
- [x] Provider Adapter Specification

### Phase 2: Core Implementation ✅
- [x] Project scaffolding (Go module, config, slog, graceful shutdown)
- [x] Database migrations with seed data (golang-migrate format)
- [x] Repository layer (sqlc — type-safe queries for all 5 tables)
- [x] Generic JSON adapter (URL templates, JSONPath, currency code mapping)
- [x] Adapter registry (resolve by provider name)
- [x] Polling engine (per-pair goroutines, failover, health tracking)
- [x] gRPC server (GetRate, GetRates, ListSupportedPairs)
- [x] HTTP health endpoints (/healthz, /readyz with JSON status)

### Phase 3: Testing & QA 📋
- [ ] E2E testing (grpcurl, provider failover, staleness scenarios)
- [ ] Unit tests (adapters, polling, repository)
- [ ] Edge case verification (unknown currency, DB disconnect)

### Phase 4: Deployment 📋
- [ ] Dockerfile (multi-stage build)
- [ ] docker-compose alongside Expense Tracker
- [ ] Demo environment deployment (Hetzner VPS)
- [ ] Health endpoint monitoring

### Phase 5: Integration 📋
- [x] Expense Tracker gRPC client integration
- [ ] GetRateHistory method (v2)

### Backlog
- **Intraday rate source** — all current providers publish one rate per day. Trigger: after the Crypto Account Service roadmap is finished (owner decision 2026-10-05).
- **Frankfurter API v2** — v1 is deprecated but stays available. Trigger: a v1 shutdown notice or the next change of the Frankfurter provider row.
- **fawazahmed0 jsDelivr caching** — individual `*.min.json` files can be served weeks out of date (observed 2026-10-05: `gbp.min.json` dated 2026-08-31) and `is_outdated` does not show it. Trigger: before CRS is deployed for a consumer outside local development.

## 📄 License

This project is licensed under the **MIT License**.
See the [`LICENSE`](LICENSE) file for details.

## 👤 Author

**Igor Kudinov**

This project is part of my professional portfolio demonstrating:
- Requirements analysis and documentation (BRD, SRS)
- Microservice architecture design
- gRPC API design with Protocol Buffers
- Database-driven configuration patterns
- Provider adapter architecture (strategy pattern)
- Go backend development
- Inter-service communication in a microservice ecosystem

## 🔗 Links

- [GitHub Repository](https://github.com/DigitLock/currency-rate-service)
- [Expense Tracker](https://github.com/DigitLock/expense-tracker) (primary consumer)
- Portfolio: [portfolio.digitlock.systems](https://portfolio.digitlock.systems/)