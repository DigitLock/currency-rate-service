# Currency Rate Service

Autonomous microservice that collects, stores, and serves live and historical currency exchange rates via gRPC. Part of the [Expense Tracker](https://github.com/DigitLock/expense-tracker) ecosystem.

## 🎯 Project Status

- ✅ **Business Requirements** - Complete (BRD v1.0)
- ✅ **System Requirements** - Complete (SRS v1.0)
- 📋 **Implementation** - Planned

## ✨ Features

- 💱 **Multi-currency rates** (RSD↔EUR, RSD↔USD, EUR↔USD)
- 🔄 **Automated polling** with configurable per-pair intervals
- 🛡️ **Provider failover** — primary/backup with automatic switchover
- ⚠️ **Staleness tracking** — last known rate always available, flagged when outdated
- 🗄️ **Database-driven config** — add pairs and providers at runtime, no restart
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
- **Proto Tooling**: protoc, buf (linting/breaking changes)

### Infrastructure
- **Deployment**: Docker (standalone container)
- **Server**: Hetzner VPS (same host as Expense Tracker)
- **DNS**: Cloudflare (HTTP health endpoint only — gRPC via direct IP)

## 📡 gRPC API

**Service:** `currency_rate.v1.CurrencyRateService`

Proto file: `proto/currency_rate/v1/currency_rate.proto`

| Method | Description |
|--------|-------------|
| `GetRate` | Current rate for a single currency pair |
| `GetRates` | Current rates for multiple targets relative to a base currency |
| `ListSupportedPairs` | All configured pairs with polling metadata |
| `GetRateHistory` | Historical rates for a pair within a date range (may be deferred to v2) |

**Authentication**: None — exchange rates are public data.

**Testing with grpcurl:**
```bash
# List supported pairs
grpcurl -plaintext localhost:50052 \
  currency_rate.v1.CurrencyRateService/ListSupportedPairs

# Get single rate
grpcurl -plaintext -d '{"from_currency":"RSD","to_currency":"EUR"}' \
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

See the [SRS](docs/currency_rate_service_srs.md) Section 2.4 for full schema, constraints, and indexes.

## 🌐 External Rate Providers (v1)

| Provider | Pairs | Rate Limits | Auth |
|----------|-------|-------------|------|
| [fawazahmed0](https://github.com/fawazahmed0/exchange-api) (CDN) | RSD↔EUR, RSD↔USD, EUR↔USD | None | None |
| [fawazahmed0](https://github.com/fawazahmed0/exchange-api) (pages.dev fallback) | RSD↔EUR, RSD↔USD, EUR↔USD | None | None |
| [ExchangeRate-API](https://www.exchangerate-api.com/docs/free) (open access) | RSD↔EUR, RSD↔USD, EUR↔USD | ~1/day recommended | None |
| [Frankfurter](https://frankfurter.dev/) (ECB data) | EUR↔USD only | None | None |

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

### Business (database-driven)

Currency pairs, providers, polling intervals, and pair-provider assignments are managed in the database. Changes take effect on the next polling cycle — no service restart required.

## 📚 Documentation

Located in `docs/`:

- [`currency_rate_service_brd.md`](docs/currency_rate_service_brd.md) – Business Requirements Document (12 BRs, stakeholders, risks, benefits)
- [`currency_rate_service_srs.md`](docs/currency_rate_service_srs.md) – System Requirements Specification (gRPC API contract, 4 use cases, 5-table data model, 6 ADRs, provider adapter spec)

## 📋 Project Structure

```
currency-rate-service/
├── docs/                    # Business and system requirements
│   ├── currency_rate_service_brd.md
│   └── currency_rate_service_srs.md
├── cmd/
│   └── server/              # Application entrypoint
├── internal/
│   ├── config/              # Environment and DB config loading
│   ├── grpc/
│   │   ├── pb/              # Generated protobuf Go code
│   │   └── server/          # gRPC handlers
│   ├── health/              # HTTP health endpoints (/healthz, /readyz)
│   ├── polling/             # Scheduler and polling engine
│   ├── provider/
│   │   ├── adapter/         # Generic JSON adapter
│   │   └── registry/        # Adapter registry (generic + custom)
│   └── repository/          # Database queries (sqlc)
├── migrations/              # Versioned SQL migrations
├── proto/
│   └── currency_rate/v1/    # Proto source files
├── Dockerfile
├── docker-compose.yml
├── buf.yaml                 # buf lint configuration
├── Makefile                 # Proto codegen, build, migrate
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

## 🚀 Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL 15+
- protoc + protoc-gen-go + protoc-gen-go-grpc
- Docker (for deployment)

### Development

```bash
# Clone
git clone https://github.com/DigitLock/currency-rate-service.git
cd currency-rate-service

# Create database
createdb currency_rates_dev

# Run migrations
make migrate-up

# Generate proto (if changed)
make proto

# Run
DATABASE_URL="postgres://user:pass@localhost:5432/currency_rates_dev?sslmode=disable" \
  go run ./cmd/server
```

### Docker

```bash
docker build -t currency-rate-service .
docker run -d \
  -e DATABASE_URL="postgres://user:pass@host:5432/currency_rates?sslmode=disable" \
  -p 50052:50052 \
  -p 8090:8090 \
  currency-rate-service
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

## 🎨 Demo

**Demo environment** (Hetzner VPS):

| Service | Address |
|---------|---------|
| gRPC API | `46.224.29.194:50052` (plaintext) |
| Health | `http://46.224.29.194:8090/readyz` |

```bash
grpcurl -plaintext 46.224.29.194:50052 \
  currency_rate.v1.CurrencyRateService/ListSupportedPairs
```

**Note**: gRPC is accessible via direct IP only — Cloudflare free plan does not proxy gRPC traffic.

## 🚀 Roadmap

### Phase 1: Documentation ✅
- [x] Business Requirements Document (BRD v1.0)
- [x] System Requirements Specification (SRS v1.0)
- [x] Architecture Decision Records (6 ADRs)
- [x] Provider Adapter Specification

### Phase 2: Core Implementation 📋
- [ ] Project scaffolding (go.mod, Makefile, Dockerfile)
- [ ] Database migrations and seed data
- [ ] Repository layer (sqlc)
- [ ] Generic JSON adapter
- [ ] Adapter registry
- [ ] Polling engine (scheduler, failover, health tracking)
- [ ] gRPC server (4 methods)
- [ ] HTTP health endpoints

### Phase 3: Testing & QA 📋
- [ ] Unit tests (adapters, polling, repository)
- [ ] Integration tests (polling → storage → retrieval)
- [ ] grpcurl verification for all methods
- [ ] Provider failover scenarios

### Phase 4: Deployment 📋
- [ ] Docker build and compose
- [ ] Demo environment deployment
- [ ] Expense Tracker integration (gRPC client)
- [ ] Monitoring and alerting

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
- Portfolio: [digitlock.systems](https://digitlock.systems)