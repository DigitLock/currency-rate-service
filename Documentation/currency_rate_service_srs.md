Currency Rate Service - System Requirements Specification

---

# 1. Introduction

This System Requirements Specification (SRS) document provides detailed technical requirements for **v1** of the Currency Rate Service — an autonomous microservice that collects, stores, and serves live and historical currency exchange rates to applications within the Expense Tracker ecosystem via gRPC.

**Document Purpose**

This document serves as the primary technical specification for v1 implementation, defining the gRPC API contract, data model, polling engine behavior, provider adapter architecture, deployment configuration, and non-functional constraints that enable centralized exchange rate management for the ecosystem.

**Scope**

The SRS covers the v1 implementation with focus on essential functionality:

- **Core Functionality**: Automated rate collection from external providers, current and historical rate storage, gRPC API for rate retrieval
- **Technical Architecture**: Standalone Go microservice with gRPC API and dedicated PostgreSQL database
- **Currency Pairs**: Three fiat pairs — RSD↔EUR, RSD↔USD, EUR↔USD
- **Provider Architecture**: Hybrid adapter system — generic JSON adapter (config-driven) and custom adapter interface (code)
- **Configuration**: Database-driven business configuration, config file for system settings only
- **Cryptocurrency Support**: Out of scope for v1; architecture designed to support it in future iterations

**Target Audience**

- Software developer implementing the service
- Business stakeholder (Igor Kudinov) for requirement validation
- QA engineer for testing specification
- Future maintainers and consumers of the gRPC API

**Related Documents**

- **Business Requirements Document (BRD)**: Currency Rate Service BRD v1.0
- **Expense Tracker BRD**: Expense Tracker BRD v1.0 (BR-5: Multi-Currency Support)
- **Expense Tracker SRS**: Expense Tracker SRS MVP v1.0
- **gRPC Integration TDD**: Expense Tracker TDD gRPC Integration v1.0 (ADR-6: Inter-Service Communication)

**Technology Stack**

- **Backend**: Go (Golang) with gRPC server
- **Database**: PostgreSQL (dedicated database for rate storage and configuration)
- **API Protocol**: gRPC with Protocol Buffers (unauthenticated)
- **Provider Communication**: HTTP/REST clients for external rate APIs
- **Deployment**: Standalone Docker container on Hetzner VPS
- **Tooling**: protoc/buf (proto codegen), sqlc (type-safe SQL), pgx/v5 (PostgreSQL driver)

---
<br />


# 2. Functional Requirements

## 2.1 API

### 2.1.1 System Architecture

**Components Diagram**

```mermaid
graph TB
    subgraph "External Rate Providers"
        EP1["fawazahmed0<br/>(CDN/REST)"]
        EP2["ExchangeRate-API<br/>(REST)"]
        EP3["Frankfurter<br/>(REST, EUR only)"]
    end

    subgraph "Currency Rate Service"
        subgraph "Polling Subsystem"
            SCH["Scheduler"]
            AR["Adapter Registry"]
            GA["Generic JSON<br/>Adapter"]
            CA["Custom<br/>Adapter(s)"]
            HT["Health Tracker"]
        end

        subgraph "Data Layer"
            CL["Config Loader<br/>(reads DB)"]
            REPO["Repository<br/>(sqlc/pgx)"]
            DB[(PostgreSQL)]
        end

        subgraph "API Layer"
            GRPC["gRPC Server<br/>(CurrencyRateService)"]
            HEALTH["Health Endpoint<br/>(HTTP)"]
        end
    end

    EP1 & EP2 & EP3 -->|"HTTP response"| GA
    GA & CA --> AR
    SCH -->|"triggers per pair"| AR
    AR -->|"fetch result"| REPO
    AR -->|"status update"| HT
    HT --> REPO
    CL -->|"pairs, providers,<br/>intervals"| SCH
    CL --> DB
    REPO --> DB
    GRPC -->|"query rates"| REPO
    HEALTH -->|"query status"| HT

    ET["Expense Tracker<br/>Backend"] -->|"gRPC"| GRPC
    IG["Invoice Generator<br/>(future)"] -.->|"gRPC"| GRPC
```

**Sequence Diagram — End-to-End Flow**

```mermaid
sequenceDiagram
    participant Scheduler
    participant AdapterRegistry
    participant Provider as External Provider
    participant Repo as Repository
    participant DB as PostgreSQL
    participant Consumer as Expense Tracker

    Note over Scheduler: Polling interval elapsed for RSD→EUR

    Scheduler->>AdapterRegistry: FetchRate(RSD, EUR)
    AdapterRegistry->>Provider: HTTP GET (primary provider)
    Provider-->>AdapterRegistry: JSON response with rate

    AdapterRegistry->>Repo: StoreRate(pair, rate, source, timestamp)
    Repo->>DB: Store rate record

    Note over Consumer: User views transactions

    Consumer->>Repo: GetRate(RSD, EUR) via gRPC
    Repo->>DB: Query latest rate for pair
    DB-->>Repo: rate + metadata
    Repo-->>Consumer: Rate { value, updated_at, is_outdated, source }
```

### 2.1.2 gRPC Service: CurrencyRateService

**Proto File**

```protobuf
syntax = "proto3";

package currency_rate.v1;

option go_package = "github.com/DigitLock/currency-rate-service/internal/grpc/pb";

import "google/protobuf/timestamp.proto";

// CurrencyRateService provides exchange rate data to any consumer without authentication.
service CurrencyRateService {
  // Returns the current exchange rate for a single currency pair.
  rpc GetRate(GetRateRequest) returns (GetRateResponse);

  // Returns current rates for multiple target currencies relative to a base currency.
  rpc GetRates(GetRatesRequest) returns (GetRatesResponse);

  // Returns all configured currency pairs with their polling configuration.
  rpc ListSupportedPairs(ListSupportedPairsRequest) returns (ListSupportedPairsResponse);

  // Returns historical rates for a currency pair within a date range.
  // Note: may be deferred to v2 implementation.
  rpc GetRateHistory(GetRateHistoryRequest) returns (GetRateHistoryResponse);
}

// --- Common Messages ---

message Rate {
  string from_currency = 1;          // ISO 4217 currency code (e.g., "RSD")
  string to_currency = 2;            // ISO 4217 currency code (e.g., "EUR")
  double rate = 3;                   // Exchange rate value (see note on precision below)
  google.protobuf.Timestamp updated_at = 4;  // When this rate was fetched from the provider
  bool is_outdated = 5;              // True if the most recent polling cycle failed (staleness flag)
  string source_provider = 6;        // Name of the provider that supplied this rate
}

message CurrencyPair {
  string from_currency = 1;
  string to_currency = 2;
  int32 polling_interval_seconds = 3;
  bool is_active = 4;
}

// --- GetRate ---

message GetRateRequest {
  string from_currency = 1;          // Required. ISO 4217 code.
  string to_currency = 2;            // Required. ISO 4217 code.
}

message GetRateResponse {
  Rate rate = 1;
}

// --- GetRates ---

message GetRatesRequest {
  string base_currency = 1;                    // Required. Base currency code.
  repeated string target_currencies = 2;       // Required. One or more target currency codes.
}

message GetRatesResponse {
  repeated Rate rates = 1;
}

// --- ListSupportedPairs ---

message ListSupportedPairsRequest {}

message ListSupportedPairsResponse {
  repeated CurrencyPair pairs = 1;
}

// --- GetRateHistory ---

message GetRateHistoryRequest {
  string from_currency = 1;
  string to_currency = 2;
  google.protobuf.Timestamp start_date = 3;   // Required. Start of date range (inclusive).
  google.protobuf.Timestamp end_date = 4;     // Required. End of date range (inclusive).
}

message GetRateHistoryResponse {
  repeated Rate rates = 1;                     // Ordered by updated_at ascending.
}
```

> **Precision note:** The proto field `Rate.rate` uses `double` (IEEE 754 64-bit float) because Protocol Buffers does not have a native decimal type. The database stores rates as `NUMERIC(20,10)` for exact precision. Conversion from `NUMERIC` to `double` may introduce minor floating-point rounding (e.g., 15th decimal place), which is acceptable for the service's informational purpose — rates are approximate, not financial-grade.

**Authorization**

:negative_squared_cross_mark: _No authorization required. Exchange rates are public data. All gRPC methods are accessible without authentication._

**gRPC Error Code Mapping**

| Scenario | gRPC Status Code | Description |
|----------|-----------------|-------------|
| Unknown currency code | `3 INVALID_ARGUMENT` | Currency code not recognized (not ISO 4217) |
| Pair not configured | `5 NOT_FOUND` | Requested currency pair is not in the configured pairs list |
| No rate data yet | `5 NOT_FOUND` | Pair is configured but no rate has been fetched yet |
| Internal error | `13 INTERNAL` | Database error or unexpected failure |
| Empty required field | `3 INVALID_ARGUMENT` | from_currency or to_currency is empty |

---

#### 2.1.2.1 GetRate

**Description**

Returns the current exchange rate for a single currency pair. The response includes staleness metadata — if the most recent polling cycle failed for both primary and backup providers, `is_outdated` is set to `true` and the last known rate is returned.

**Request Parameters**

| Parameter | Type | Required | Description | Example |
|-----------|------|----------|-------------|---------|
| from_currency | string | Yes | Source currency (ISO 4217) | RSD |
| to_currency | string | Yes | Target currency (ISO 4217) | EUR |

**Request Example**

```json
{
  "from_currency": "RSD",
  "to_currency": "EUR"
}
```

**Response Parameters**

| Parameter | Type | Required | Description | Example |
|-----------|------|----------|-------------|---------|
| rate.from_currency | string | Yes | Source currency code | RSD |
| rate.to_currency | string | Yes | Target currency code | EUR |
| rate.rate | double | Yes | Exchange rate value | 0.00853 |
| rate.updated_at | Timestamp | Yes | When the rate was fetched | 2026-03-21T16:00:00Z |
| rate.is_outdated | bool | Yes | Staleness flag | false |
| rate.source_provider | string | Yes | Provider that supplied the rate | fawazahmed0 |

**Response Example**

```json
{
  "rate": {
    "from_currency": "RSD",
    "to_currency": "EUR",
    "rate": 0.00853,
    "updated_at": "2026-03-21T16:00:00Z",
    "is_outdated": false,
    "source_provider": "fawazahmed0"
  }
}
```

**Sequence Diagram**

```mermaid
sequenceDiagram
    participant Consumer
    participant gRPC as gRPC Server
    participant Repo as Repository
    participant DB as PostgreSQL

    Consumer->>gRPC: GetRate(from=RSD, to=EUR)
    gRPC->>gRPC: Validate currency codes
    gRPC->>Repo: GetLatestRate(pair_id)
    Repo->>DB: Query latest rate for pair
    DB-->>Repo: Rate row
    Repo-->>gRPC: Rate entity
    gRPC-->>Consumer: GetRateResponse { rate }
```

---

#### 2.1.2.2 GetRates

**Description**

Returns current rates for multiple target currencies relative to a single base currency. Useful when the consumer needs several conversions at once (e.g., Expense Tracker displaying a transaction in RSD with EUR and USD equivalents). Each rate in the response includes its own staleness flag independently.

**Request Parameters**

| Parameter | Type | Required | Description | Example |
|-----------|------|----------|-------------|---------|
| base_currency | string | Yes | Base currency (ISO 4217) | RSD |
| target_currencies | repeated string | Yes | One or more target currencies | ["EUR", "USD"] |

**Request Example**

```json
{
  "base_currency": "RSD",
  "target_currencies": ["EUR", "USD"]
}
```

**Response Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| rates | repeated Rate | Yes | One Rate per requested target. Missing pairs are omitted (not an error). |

**Response Example**

```json
{
  "rates": [
    {
      "from_currency": "RSD",
      "to_currency": "EUR",
      "rate": 0.00853,
      "updated_at": "2026-03-21T16:00:00Z",
      "is_outdated": false,
      "source_provider": "fawazahmed0"
    },
    {
      "from_currency": "RSD",
      "to_currency": "USD",
      "rate": 0.00928,
      "updated_at": "2026-03-21T16:00:00Z",
      "is_outdated": false,
      "source_provider": "fawazahmed0"
    }
  ]
}
```

---

#### 2.1.2.3 ListSupportedPairs

**Description**

Returns all currency pairs configured in the service. This allows consumers to discover which pairs are available before making rate requests.

**Request Parameters**

None. Empty request message.

**Response Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| pairs | repeated CurrencyPair | Yes | All configured pairs with metadata |
| pairs[].from_currency | string | Yes | Source currency code |
| pairs[].to_currency | string | Yes | Target currency code |
| pairs[].polling_interval_seconds | int32 | Yes | How often this pair is polled |
| pairs[].is_active | bool | Yes | Whether polling is currently enabled |

**Response Example**

```json
{
  "pairs": [
    { "from_currency": "RSD", "to_currency": "EUR", "polling_interval_seconds": 3600, "is_active": true },
    { "from_currency": "RSD", "to_currency": "USD", "polling_interval_seconds": 3600, "is_active": true },
    { "from_currency": "EUR", "to_currency": "USD", "polling_interval_seconds": 3600, "is_active": true }
  ]
}
```

---

#### 2.1.2.4 GetRateHistory

> **Note:** This method may be deferred to v2 implementation. The proto definition and data model support it from v1, but the handler implementation is optional for the initial release.

**Description**

Returns historical exchange rates for a currency pair within a specified date range. Results are ordered by timestamp ascending.

**Request Parameters**

| Parameter | Type | Required | Description | Example |
|-----------|------|----------|-------------|---------|
| from_currency | string | Yes | Source currency (ISO 4217) | EUR |
| to_currency | string | Yes | Target currency (ISO 4217) | USD |
| start_date | Timestamp | Yes | Start of range (inclusive) | 2026-01-01T00:00:00Z |
| end_date | Timestamp | Yes | End of range (inclusive) | 2026-03-21T23:59:59Z |

**Response Parameters**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| rates | repeated Rate | Yes | Historical rates ordered by updated_at ascending |

**Response Example**

```json
{
  "rates": [
    {
      "from_currency": "EUR",
      "to_currency": "USD",
      "rate": 1.0842,
      "updated_at": "2026-01-01T16:00:00Z",
      "is_outdated": false,
      "source_provider": "frankfurter"
    },
    {
      "from_currency": "EUR",
      "to_currency": "USD",
      "rate": 1.0911,
      "updated_at": "2026-01-02T16:00:00Z",
      "is_outdated": false,
      "source_provider": "frankfurter"
    }
  ]
}
```

---
<br />


## 2.2 User Interface

> Not applicable — Currency Rate Service is a headless microservice with no user-facing interface. Consumers interact via gRPC API only.

---
<br />


## 2.3 Use Cases

### 2.3.1 Scheduled Rate Polling

**Sequence Diagram**

```mermaid
sequenceDiagram
    participant Scheduler
    participant Config as Config Loader
    participant AR as Adapter Registry
    participant Primary as Primary Provider
    participant Backup as Backup Provider
    participant HT as Health Tracker
    participant Repo as Repository
    participant DB as PostgreSQL

    Scheduler->>Config: Get active pairs and intervals
    Config->>DB: Load pairs + provider assignments
    DB-->>Config: Pairs with providers

    loop For each pair where interval elapsed
        Scheduler->>AR: FetchRate(pair, primary_provider)
        AR->>Primary: HTTP GET
        alt Primary succeeds
            Primary-->>AR: Rate data
            AR->>Repo: StoreRate(pair, rate, provider, timestamp)
            Repo->>DB: Store rate (is_outdated=false)
            AR->>HT: RecordSuccess(provider, pair)
            HT->>DB: Update health status
        else Primary fails
            Primary-->>AR: Error / timeout
            AR->>HT: RecordFailure(provider, pair)
            AR->>AR: Attempt backup provider
            AR->>Backup: HTTP GET
            alt Backup succeeds
                Backup-->>AR: Rate data
                AR->>Repo: StoreRate(pair, rate, backup_provider, timestamp)
                Repo->>DB: Store rate (is_outdated=false)
                AR->>HT: RecordSuccess(backup, pair)
            else Backup also fails
                Backup-->>AR: Error / timeout
                AR->>HT: RecordFailure(backup, pair)
                AR->>Repo: MarkOutdated(pair)
                Repo->>DB: Flag latest rate as outdated
            end
        end
    end
```

**Algorithm**

1. Scheduler checks all active currency pairs and their polling intervals
2. For each pair where the interval has elapsed since the last poll:
   a. Resolve the primary provider from `pair_provider_config`
   b. Call the provider adapter's `FetchRate()` method
   c. If successful: store the rate with `is_outdated=false`, record success in health tracker
   d. If failed: record failure in health tracker, resolve backup provider
   e. Call backup provider's `FetchRate()` method
   f. If successful: store rate from backup with `is_outdated=false`, record success
   g. If failed: mark the current rate for this pair as `is_outdated=true`, record failure

**Preconditions**
- Service is running and connected to PostgreSQL
- At least one currency pair is configured and active in the database
- At least one provider is assigned to each active pair

**Trigger**
Polling interval elapsed for a currency pair (checked by the scheduler on a tick cycle).

**Basic Flow**
Primary provider responds successfully → rate stored → health updated → done.

**Exception Paths**
- Primary provider returns HTTP error or times out → failover to backup
- Backup provider also fails → staleness flag set on current rate
- Database write fails → error logged, rate not stored, next cycle will retry
- No provider assigned to pair → logged as configuration error, pair skipped

**Acceptance Criteria**
- Rate is stored with source provider name, fetch timestamp, and correct staleness flag
- Historical record is created (new INSERT, not UPDATE of previous rate)
- Provider health is updated after every attempt (success or failure)

**Postconditions**
- Current rate for the pair is either fresh (new rate stored) or flagged as outdated
- A new historical record exists in the `rates` table
- Provider health counters are updated

---

### 2.3.2 Provider Failover

**Sequence Diagram**

```mermaid
sequenceDiagram
    participant AR as Adapter Registry
    participant Primary as Primary Provider
    participant Backup as Backup Provider
    participant HT as Health Tracker

    AR->>Primary: FetchRate(RSD→EUR)
    Primary-->>AR: Error (HTTP 503)
    AR->>HT: RecordFailure(primary, RSD→EUR)
    HT-->>AR: consecutive_failures = 3

    AR->>Backup: FetchRate(RSD→EUR)
    alt Backup succeeds
        Backup-->>AR: Rate = 0.00853
        AR->>HT: RecordSuccess(backup, RSD→EUR)
        AR-->>AR: Return rate (source = backup)
    else Backup fails
        Backup-->>AR: Error (timeout)
        AR->>HT: RecordFailure(backup, RSD→EUR)
        AR-->>AR: Return failover_exhausted
    end
```

**Algorithm**

1. Primary provider call fails (HTTP error, timeout, parse error)
2. Health tracker increments `consecutive_failures` for the primary provider
3. Adapter registry resolves the backup provider for this pair from `pair_provider_config`
4. Backup provider's `FetchRate()` is called
5. If backup succeeds: rate returned with backup as source, health updated
6. If backup fails: both providers exhausted, return `failover_exhausted` signal to scheduler

**Preconditions**
- Primary provider has been attempted and failed
- Backup provider is configured for this currency pair

**Trigger**
Primary provider returns an error or does not respond within the configured timeout.

**Basic Flow**
Primary fails → backup succeeds → rate stored with backup source.

**Exception Paths**
- No backup configured → failover skipped, staleness flag set immediately
- Backup returns invalid data (parse error) → treated as failure

**Acceptance Criteria**
- Failover occurs within the same polling cycle — no additional delay beyond the backup request
- The rate stored after failover correctly identifies the backup as the source provider
- Health tracker accurately reflects failures for both providers independently

**Postconditions**
- Rate stored from backup source, OR staleness flag set if both failed
- Health records updated for both providers

---

### 2.3.3 Rate Retrieval by Consumer

**Sequence Diagram**

```mermaid
sequenceDiagram
    participant Consumer as Expense Tracker
    participant gRPC as gRPC Server
    participant Repo as Repository
    participant DB as PostgreSQL

    Consumer->>gRPC: GetRate(RSD, EUR)
    gRPC->>gRPC: Validate request
    gRPC->>Repo: FindPairByCode(RSD, EUR)
    Repo->>DB: Look up active pair
    DB-->>Repo: pair_id

    gRPC->>Repo: GetLatestRate(pair_id)
    Repo->>DB: Query latest rate for pair
    DB-->>Repo: Rate row with metadata
    Repo-->>gRPC: Rate entity
    gRPC-->>Consumer: GetRateResponse
```

**Algorithm**

1. Receive gRPC request with `from_currency` and `to_currency`
2. Validate: both fields non-empty, recognized currency codes
3. Look up the currency pair in `currency_pairs` table (must be active)
4. If pair not found: return `NOT_FOUND`
5. Query the latest rate for this pair from the `rates` table
6. If no rate exists yet: return `NOT_FOUND` with descriptive message
7. Return rate with all metadata (value, timestamp, staleness flag, source)

**Preconditions**
- Consumer has established a gRPC channel to the service
- Currency pair is configured and active

**Trigger**
Consumer calls `GetRate` or `GetRates` gRPC method.

**Basic Flow**
Request received → pair resolved → latest rate queried → response returned with metadata.

**Exception Paths**
- Unknown currency code → `INVALID_ARGUMENT`
- Pair not configured → `NOT_FOUND`
- Pair configured but no rate fetched yet (service just started) → `NOT_FOUND`
- Database connection error → `INTERNAL`

**Acceptance Criteria**
- Response includes rate value, timestamp, `is_outdated` flag, and source provider name
- Response time <50ms (rate served from local database)
- No external API calls during rate retrieval

**Postconditions**
None — read-only operation, no state changes.

---

### 2.3.4 Staleness Management

**Sequence Diagram**

```mermaid
sequenceDiagram
    participant Scheduler
    participant AR as Adapter Registry
    participant Repo as Repository
    participant DB as PostgreSQL
    participant Consumer as Expense Tracker

    Note over Scheduler: Polling cycle for RSD→EUR

    Scheduler->>AR: FetchRate(RSD→EUR, primary)
    AR-->>Scheduler: Failed
    Scheduler->>AR: FetchRate(RSD→EUR, backup)
    AR-->>Scheduler: Failed

    Scheduler->>Repo: MarkOutdated(pair)
    Repo->>DB: Flag latest rate as outdated

    Note over Consumer: Later — user views transactions

    Consumer->>Repo: GetRate(RSD, EUR) via gRPC
    Repo->>DB: Query latest rate
    DB-->>Repo: rate=0.00853, is_outdated=true
    Repo-->>Consumer: Rate { rate=0.00853, is_outdated=true }

    Note over Consumer: Shows "rate may be outdated" in UI

    Note over Scheduler: Next polling cycle succeeds

    Scheduler->>AR: FetchRate(RSD→EUR, primary)
    AR-->>Scheduler: Rate = 0.00861
    Scheduler->>Repo: StoreRate(pair, 0.00861, ...)
    Repo->>DB: Store new rate (is_outdated=false)

    Note over Consumer: Next request returns fresh rate
    Consumer->>Repo: GetRate(RSD, EUR) via gRPC
    Repo->>DB: Query latest rate
    DB-->>Repo: rate=0.00861, is_outdated=false
    Repo-->>Consumer: Rate { rate=0.00861, is_outdated=false }
```

**Algorithm — Staleness Flag Lifecycle**

1. **Flag set to `true`:** When a polling cycle fails for both primary and backup providers, the most recent rate record for this pair is updated with `is_outdated=true`
2. **Flag remains `true`:** On subsequent consumer requests, the last known rate is returned with `is_outdated=true` — the consumer decides how to handle it (show with warning, hide, block operation)
3. **Flag reset to `false`:** On the next successful polling cycle, a new rate record is inserted with `is_outdated=false` — this becomes the latest rate, effectively replacing the outdated one

**Preconditions**
- At least one rate exists for the pair (from a previous successful poll)

**Trigger**
Both primary and backup providers fail during a polling cycle.

**Basic Flow**
Both providers fail → `is_outdated=true` set on latest rate → consumer receives flagged rate → next successful poll inserts fresh rate with `is_outdated=false`.

**Exception Paths**
- No previous rate exists (first poll ever fails) → no rate to flag; GetRate returns `NOT_FOUND`
- Multiple consecutive polling cycles fail → flag remains `true`, rate value unchanged, timestamp reflects last successful fetch

**Acceptance Criteria**
- `is_outdated` accurately reflects whether the rate was updated in the most recent polling cycle
- Last known rate is always available (never deleted due to staleness)
- Consumer can distinguish fresh from outdated rates using the flag
- Successful poll after an outage clears the staleness flag

**Postconditions**
- Rate remains queryable with `is_outdated=true` until next successful poll
- No data is lost — outdated rate becomes a historical record once a fresh rate is inserted

---
<br />


## 2.4 Data Model

### 2.4.1 Database Schema Overview

```mermaid
erDiagram
    currency_pairs ||--o{ pair_provider_config : "has providers"
    currency_pairs ||--o{ rates : "has rates"
    currency_pairs ||--o{ provider_health : "has health records"
    providers ||--o{ pair_provider_config : "assigned to pairs"
    providers ||--o{ rates : "source of"
    providers ||--o{ provider_health : "tracked per pair"

    currency_pairs {
        bigserial id PK
        varchar from_currency
        varchar to_currency
        int polling_interval_seconds
        boolean is_active
        timestamp created_at
        timestamp updated_at
    }

    providers {
        bigserial id PK
        varchar name
        varchar adapter_type
        text base_url
        varchar rate_json_path
        jsonb currency_code_mapping
        varchar api_key
        boolean is_active
        timestamp created_at
        timestamp updated_at
    }

    pair_provider_config {
        bigserial id PK
        bigint currency_pair_id FK
        bigint provider_id FK
        varchar priority
        boolean is_active
        timestamp created_at
    }

    rates {
        bigserial id PK
        bigint currency_pair_id FK
        bigint source_provider_id FK
        numeric rate
        boolean is_outdated
        timestamp fetched_at
        timestamp created_at
    }

    provider_health {
        bigserial id PK
        bigint provider_id FK
        bigint currency_pair_id FK
        timestamp last_success_at
        timestamp last_failure_at
        int consecutive_failures
        text last_error_message
        timestamp updated_at
    }
```

### 2.4.2 Table: currency_pairs

**Description**
Configured currency pairs. This is the primary business configuration table (per BR-12). Adding a new pair is a database operation — no code changes or service restart required.

**Data Model**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | BIGSERIAL | Yes | Primary key (auto-increment) |
| from_currency | VARCHAR(10) | Yes | Source currency code (ISO 4217, e.g., "RSD") |
| to_currency | VARCHAR(10) | Yes | Target currency code (ISO 4217, e.g., "EUR") |
| polling_interval_seconds | INTEGER | Yes | How often to poll for this pair (default: 3600) |
| is_active | BOOLEAN | Yes | Whether polling is enabled for this pair (default: true) |
| created_at | TIMESTAMP | Yes | Record creation time |
| updated_at | TIMESTAMP | Yes | Last modification time |

**Constraints**

| Constraint | Type | Description |
|-----------|------|-------------|
| currency_pairs_pkey | PRIMARY KEY | id |
| currency_pairs_pair_unique | UNIQUE | (from_currency, to_currency) |
| currency_pairs_interval_check | CHECK | polling_interval_seconds >= 60 |
| currency_pairs_from_check | CHECK | from_currency ~ '^[A-Z]{3,5}$' |
| currency_pairs_to_check | CHECK | to_currency ~ '^[A-Z]{3,5}$' |

> Currency code regex allows 3-5 uppercase characters to support both fiat (3 chars: RSD, EUR, USD) and future crypto codes (4-5 chars: USDT, USDC).

### 2.4.3 Table: providers

**Description**
Available rate provider definitions. Each provider has an adapter type that determines how the service communicates with it: `generic_json` (config-driven) or `custom` (code-level adapter).

**Data Model**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | BIGSERIAL | Yes | Primary key (auto-increment) |
| name | VARCHAR(100) | Yes | Unique provider identifier (e.g., "fawazahmed0") |
| adapter_type | VARCHAR(20) | Yes | "generic_json" or "custom" |
| base_url | TEXT | Yes | URL template with placeholders (e.g., `https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/{from}.json`) |
| rate_json_path | VARCHAR(255) | No | JSONPath to rate value in response (for generic_json only, e.g., "{from}.{to}") |
| currency_code_mapping | JSONB | No | Maps internal codes to provider-specific codes (e.g., `{"RSD": "rsd", "EUR": "eur"}`) |
| api_key | VARCHAR(255) | No | API key if required by provider (nullable for free APIs) |
| is_active | BOOLEAN | Yes | Whether this provider is available for assignment (default: true) |
| created_at | TIMESTAMP | Yes | Record creation time |
| updated_at | TIMESTAMP | Yes | Last modification time |

**Constraints**

| Constraint | Type | Description |
|-----------|------|-------------|
| providers_pkey | PRIMARY KEY | id |
| providers_name_unique | UNIQUE | name |
| providers_adapter_type_check | CHECK | adapter_type IN ('generic_json', 'custom') |

### 2.4.4 Table: pair_provider_config

**Description**
Assignment of providers to currency pairs with priority (primary or backup). Each active pair should have exactly one primary and optionally one backup provider.

**Data Model**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | BIGSERIAL | Yes | Primary key (auto-increment) |
| currency_pair_id | BIGINT | Yes | FK → currency_pairs.id |
| provider_id | BIGINT | Yes | FK → providers.id |
| priority | VARCHAR(10) | Yes | "primary" or "backup" |
| is_active | BOOLEAN | Yes | Whether this assignment is active (default: true) |
| created_at | TIMESTAMP | Yes | Record creation time |

**Constraints**

| Constraint | Type | Description |
|-----------|------|-------------|
| pair_provider_config_pkey | PRIMARY KEY | id |
| pair_provider_config_unique | UNIQUE | (currency_pair_id, priority) WHERE is_active = true |
| pair_provider_config_priority_check | CHECK | priority IN ('primary', 'backup') |
| pair_provider_config_pair_fk | FOREIGN KEY | currency_pair_id → currency_pairs(id) |
| pair_provider_config_provider_fk | FOREIGN KEY | provider_id → providers(id) |

> The unique constraint is partial (active records only) — same pattern as Expense Tracker's soft-delete handling for categories.

### 2.4.5 Table: rates

**Description**
Exchange rate records — both current and historical. There is no separate "current rates" table; the current rate for a pair is simply the most recent record ordered by `fetched_at DESC`. Every successful poll inserts a new row, building historical data automatically.

**Data Model**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | BIGSERIAL | Yes | Primary key (auto-increment) |
| currency_pair_id | BIGINT | Yes | FK → currency_pairs.id |
| source_provider_id | BIGINT | Yes | FK → providers.id (which provider supplied this rate) |
| rate | NUMERIC(20,10) | Yes | Exchange rate value |
| is_outdated | BOOLEAN | Yes | Staleness flag (default: false, set to true when polling fails) |
| fetched_at | TIMESTAMP | Yes | When this rate was retrieved from the external provider |
| created_at | TIMESTAMP | Yes | Record creation time (usually same as fetched_at) |

**Constraints**

| Constraint | Type | Description |
|-----------|------|-------------|
| rates_pkey | PRIMARY KEY | id |
| rates_rate_check | CHECK | rate > 0 |
| rates_pair_fk | FOREIGN KEY | currency_pair_id → currency_pairs(id) |
| rates_provider_fk | FOREIGN KEY | source_provider_id → providers(id) |

**Indexes**

| Index | Columns | Purpose |
|-------|---------|---------|
| rates_latest_idx | (currency_pair_id, fetched_at DESC) | Efficient lookup of the most recent rate per pair |
| rates_history_idx | (currency_pair_id, fetched_at ASC) | Efficient range queries for GetRateHistory |

> `NUMERIC(20,10)` provides sufficient precision for both fiat rates (e.g., 0.0085300000 for RSD→EUR) and future crypto rates which may have many decimal places.

### 2.4.6 Table: provider_health

**Description**
Per-provider, per-pair health tracking. Updated after every polling attempt. Used by the adapter registry for failover decisions and by the health endpoint for operational monitoring.

**Data Model**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| id | BIGSERIAL | Yes | Primary key (auto-increment) |
| provider_id | BIGINT | Yes | FK → providers.id |
| currency_pair_id | BIGINT | Yes | FK → currency_pairs.id |
| last_success_at | TIMESTAMP | No | Timestamp of last successful fetch (null if never succeeded) |
| last_failure_at | TIMESTAMP | No | Timestamp of last failed fetch (null if never failed) |
| consecutive_failures | INTEGER | Yes | Number of consecutive failures (reset to 0 on success, default: 0) |
| last_error_message | TEXT | No | Error message from the most recent failure (null if last attempt succeeded) |
| updated_at | TIMESTAMP | Yes | Last update time |

**Constraints**

| Constraint | Type | Description |
|-----------|------|-------------|
| provider_health_pkey | PRIMARY KEY | id |
| provider_health_unique | UNIQUE | (provider_id, currency_pair_id) |
| provider_health_failures_check | CHECK | consecutive_failures >= 0 |
| provider_health_provider_fk | FOREIGN KEY | provider_id → providers(id) |
| provider_health_pair_fk | FOREIGN KEY | currency_pair_id → currency_pairs(id) |

### 2.4.7 Data Integrity Rules

| Table | Field | Rule | Error Message |
|-------|-------|------|---------------|
| currency_pairs | from_currency, to_currency | Unique pair combination | "Currency pair already exists" |
| currency_pairs | polling_interval_seconds | CHECK >= 60 | "Polling interval must be at least 60 seconds" |
| currency_pairs | from_currency, to_currency | CHECK regex '^[A-Z]{3,5}$' | "Invalid currency code format" |
| providers | name | UNIQUE | "Provider name already exists" |
| providers | adapter_type | CHECK IN ('generic_json', 'custom') | "Invalid adapter type" |
| pair_provider_config | (pair_id, priority) | Partial UNIQUE WHERE is_active = true | "Active assignment for this priority already exists" |
| pair_provider_config | priority | CHECK IN ('primary', 'backup') | "Priority must be primary or backup" |
| rates | rate | CHECK > 0 | "Exchange rate must be positive" |
| provider_health | consecutive_failures | CHECK >= 0 | "Failure count cannot be negative" |

**General Rules**

- **Foreign key integrity:** All FK constraints enforced at database level — no orphan records
- **No cascading deletes:** Deleting a provider or pair requires removing dependent records first (prevents accidental data loss)
- **Historical data retention:** Rate records are never deleted in v1. Future versions may introduce a retention policy (e.g., aggregate to daily averages after 1 year)
- **Soft deletes:** `is_active` flags on currency_pairs, providers, and pair_provider_config — deactivation instead of deletion, consistent with Expense Tracker patterns

---
<br />


## 2.5 Metrics and Alerts

### 2.5.1 Metrics

| Service | Metric Name | Alert Threshold | Description |
|---------|-------------|----------------|-------------|
| Polling Engine | Polling success rate (per pair) | <90% over 1 hour | Percentage of successful polling cycles per currency pair |
| Polling Engine | Polling success rate (per provider) | <80% over 1 hour | Percentage of successful fetches per provider across all pairs |
| Polling Engine | Outdated rates count | >0 for >2 polling intervals | Number of currency pairs currently flagged as outdated |
| gRPC Server | Response latency (p95) | >100ms | 95th percentile response time for rate queries |
| gRPC Server | Request count | — (informational) | Total gRPC requests per method per time period |
| Provider Health | Consecutive failures | >3 per provider per pair | Number of consecutive failed fetch attempts |
| Database | Connection pool usage | >80% capacity | Active connections relative to pool size |
| Service | Active currency pairs | — (informational) | Number of pairs with is_active=true and polling running |

### 2.5.2 Alerts

**Critical Alerts**
- **Rate staleness:** A currency pair has `is_outdated=true` for longer than 2× its polling interval — both providers are failing persistently
- **Database unavailable:** PostgreSQL connection fails for >1 minute — service cannot store or serve rates
- **gRPC server down:** Health check endpoint returns unhealthy or is unreachable for >2 minutes

**Warning Alerts**
- **Provider degradation:** A provider has >3 consecutive failures for any pair — may indicate provider outage or API change
- **Polling latency:** A polling cycle takes >30 seconds to complete (provider response slow or network issues)
- **All pairs outdated:** Every configured pair is flagged as outdated simultaneously — likely a network-level issue

**Alert Delivery (v1)**
- **Application logs:** Structured JSON logging with severity levels (INFO, WARN, ERROR)
- **Health endpoint:** HTTP endpoint returning service status and per-pair/per-provider health summary
- **Console output:** Critical alerts only (for Docker log monitoring)

---
<br />


# 3. Non-functional Requirements

## 3.1 Configuration

### 3.1.1 System Configuration (Config File / Environment Variables)

System settings are loaded at startup from environment variables (or `.env` file in development). Changes require a service restart.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `GRPC_PORT` | int | `50052` | gRPC server listen port |
| `HEALTH_HTTP_PORT` | int | `8090` | HTTP health check endpoint port |
| `DATABASE_URL` | string | — (required) | PostgreSQL connection string (e.g., `postgres://user:pass@host:5432/currency_rates?sslmode=disable`) |
| `DB_POOL_MAX_CONNS` | int | `10` | Maximum connections in the pgx pool |
| `DB_POOL_MIN_CONNS` | int | `2` | Minimum idle connections in the pgx pool |
| `LOG_LEVEL` | string | `info` | Logging level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | string | `json` | Log output format: `json` (production) or `text` (development) |
| `PROVIDER_HTTP_TIMEOUT` | duration | `10s` | HTTP client timeout for external provider requests |
| `SHUTDOWN_TIMEOUT` | duration | `15s` | Graceful shutdown deadline — time allowed for in-flight polling to complete |

**Notes**

- `GRPC_PORT` is `50052` to avoid conflict with Expense Tracker's gRPC on `50051` when both services run on the same host
- `DATABASE_URL` is the only required variable — all others have sensible defaults
- Duration values accept Go `time.Duration` format: `10s`, `1m30s`, `500ms`
- No business configuration (pairs, providers, intervals) belongs in environment variables — see Section 3.1.2

### 3.1.2 Business Configuration (Database-Driven, per BR-12)

All business configuration resides in the database and can be modified at runtime without service restart. The polling engine periodically reloads configuration from the database (reload interval: every polling tick cycle).

**Currency Pairs (`currency_pairs` table)**

| Operation | Method | Service Restart |
|-----------|--------|-----------------|
| Add a new pair | INSERT into `currency_pairs` + INSERT into `pair_provider_config` | No — picked up on next reload |
| Deactivate a pair | UPDATE `is_active = false` | No — polling stops on next reload |
| Change polling interval | UPDATE `polling_interval_seconds` | No — new interval applied on next reload |

**Providers (`providers` table)**

| Operation | Method | Service Restart |
|-----------|--------|-----------------|
| Add a generic JSON provider | INSERT into `providers` with `adapter_type = 'generic_json'` | No |
| Add a custom adapter provider | INSERT into `providers` with `adapter_type = 'custom'` | Yes — requires code deployment for the adapter |
| Deactivate a provider | UPDATE `is_active = false` | No — failover uses remaining active providers |
| Update URL or JSONPath | UPDATE `base_url` / `rate_json_path` | No — applied on next fetch |

**Pair-Provider Assignments (`pair_provider_config` table)**

| Operation | Method | Service Restart |
|-----------|--------|-----------------|
| Change primary provider for a pair | Deactivate current primary, INSERT new assignment with `priority = 'primary'` | No |
| Add backup provider | INSERT with `priority = 'backup'` | No |
| Remove backup | UPDATE `is_active = false` on the backup assignment | No |

**Configuration Reload Behavior**

- The polling engine reads active pairs and their provider assignments from the database at the start of each tick cycle
- No in-memory cache with separate TTL — configuration is always read fresh from the database alongside the polling logic
- If a configuration read fails (DB error), the previous configuration is retained until the next successful read
- Adding a new `custom` adapter type is the only operation that requires a code change and redeployment

## 3.2 General Non-functional Requirements

### 3.2.1 Performance and Scalability

**Response Time**

| Operation | Target (p95) | Constraint |
|-----------|-------------|------------|
| GetRate / GetRates | < 50ms | Served from local PostgreSQL, no external API calls at request time |
| ListSupportedPairs | < 50ms | Config data from database |
| GetRateHistory (v2) | < 200ms | Range query on indexed `fetched_at` column; larger payload |
| Health endpoint (HTTP) | < 100ms | Lightweight status check |

**Polling Engine**

| Parameter | Target | Notes |
|-----------|--------|-------|
| Concurrent pair polling | Up to N pairs polled in parallel | Each pair's polling is independent; goroutine per pair |
| External provider timeout | 10s (configurable via `PROVIDER_HTTP_TIMEOUT`) | Prevents a slow provider from blocking the polling cycle |
| Polling cycle overhead | < 5s beyond provider response time | Scheduling, DB write, health update |

**Scalability**

| Dimension | v1 Capacity | Design Ceiling |
|-----------|------------|----------------|
| Currency pairs | 3 (RSD↔EUR, RSD↔USD, EUR↔USD) | No architectural limit — adding pairs is a DB operation |
| Providers | 2–3 | No limit; adapter registry scales with provider count |
| Rate history retention | Unlimited (v1) | ~26K rows/year at hourly polling for 3 pairs; future: aggregation policy |
| Concurrent gRPC consumers | Low (2–3 services) | pgx pool sized at 10 connections; sufficient for expected consumer count |

**Database Optimization**

- **Latest rate lookup**: Covered by `rates_latest_idx` — `(currency_pair_id, fetched_at DESC)` index ensures the "current rate" query is an index-only scan
- **History range queries**: Covered by `rates_history_idx` — `(currency_pair_id, fetched_at ASC)` for efficient bounded range scans
- **No table bloat concern in v1**: At hourly polling for 3 pairs, the `rates` table grows by ~26K rows/year — negligible for PostgreSQL

### 3.2.2 Security

**API Security**

| Aspect | Policy | Rationale |
|--------|--------|-----------|
| Authentication | None | Exchange rates are public data; no user-specific information is served |
| Authorization | None | All gRPC methods are accessible to any consumer |
| Rate limiting | Not implemented in v1 | Expected consumer count is 1–2 internal services; revisit if exposed publicly |
| Input validation | Currency codes validated against `^[A-Z]{3,5}$` regex | Prevents injection; consistent with DB CHECK constraints |

**Transport Security**

| Environment | Protocol | Notes |
|-------------|----------|-------|
| Development | Plaintext gRPC | Local network, no external exposure |
| Demo (Hetzner) | Plaintext gRPC on direct IP | Same as Expense Tracker gRPC — Cloudflare free plan does not proxy gRPC (see ADR-6) |
| Production (future) | TLS | Certificate management TBD when production deployment is planned |

**Secrets Management**

| Secret | Storage | Access |
|--------|---------|--------|
| `DATABASE_URL` | Environment variable / `.env` file | Read at startup only; not logged |
| Provider API keys | `providers.api_key` column in database | Read by polling engine; not included in gRPC responses or logs |

**Logging Safety**

- Database connection strings are never logged (masked at startup)
- Provider API keys are never included in log output
- Rate values and currency codes are logged at `debug` level only (no sensitive data, but avoids log noise at `info`)
- gRPC request/response payloads are not logged at `info` level; method name and latency only

### 3.2.3 Deployment and Operations

**Container Strategy**

| Aspect | Specification |
|--------|---------------|
| Packaging | Standalone Docker container (multi-stage Go build) |
| Base image | `golang:1.25-alpine` (build), `alpine:latest` (runtime) |
| Lifecycle | Independent from Expense Tracker — separate image, separate container, separate release cycle |
| Restart policy | `unless-stopped` (Docker) or `always` (docker-compose) |

**Service Lifecycle**

| Event | Behavior |
|-------|----------|
| Startup | Connect to PostgreSQL → load configuration from DB → start polling engine → start gRPC server → start HTTP health endpoint |
| Shutdown (SIGTERM/SIGINT) | Stop accepting new gRPC connections → wait for in-flight polling cycles to complete (up to `SHUTDOWN_TIMEOUT`) → close DB pool → exit |
| DB unavailable at startup | Log error and exit with non-zero code — service cannot operate without the database |
| DB becomes unavailable at runtime | Polling cycles fail (logged as errors); gRPC queries return `INTERNAL`; service keeps running and retries on next cycle |

**Health Check**

| Endpoint | Protocol | Path | Response |
|----------|----------|------|----------|
| Liveness | HTTP | `GET /healthz` | `200 OK` if the process is running |
| Readiness | HTTP | `GET /readyz` | `200 OK` if PostgreSQL connection is healthy and at least one polling cycle has completed; `503` otherwise |

**Database Migrations**

| Aspect | Specification |
|--------|---------------|
| Tool | Versioned SQL files applied manually or via `golang-migrate` |
| Location | `migrations/` directory in repository root |
| Naming | `NNNN_description.up.sql` / `NNNN_description.down.sql` |
| Seed data | Initial migration includes seed data for v1 pairs, providers, and pair-provider assignments |
| Rollback | Every migration has a corresponding `down` file |

### 3.2.4 Observability

**Structured Logging**

| Aspect | Specification |
|--------|---------------|
| Library | Go `slog` (standard library) |
| Format | JSON in production (`LOG_FORMAT=json`), text in development (`LOG_FORMAT=text`) |
| Levels | `debug`, `info`, `warn`, `error` — controlled by `LOG_LEVEL` |
| Output | `stdout` (captured by Docker logging driver) |

**Log Events by Level**

| Level | Events |
|-------|--------|
| `info` | Service startup/shutdown, polling cycle start/complete (per pair, summary), provider failover triggered, configuration reloaded, gRPC server listening |
| `warn` | Provider fetch failed (single provider, before failover), rate flagged as outdated, configuration read failed (using previous config), slow polling cycle (>30s) |
| `error` | Both providers failed for a pair, database connection lost, gRPC server error, unrecoverable polling failure |
| `debug` | HTTP request/response to providers (URL, status code, latency), rate value stored, gRPC request details, DB query timings |

**Log Fields (Structured)**

Every log entry includes base fields for correlation and filtering:

| Field | Description | Example |
|-------|-------------|---------|
| `ts` | ISO 8601 timestamp | `2026-03-21T16:00:01Z` |
| `level` | Log level | `info` |
| `msg` | Human-readable message | `polling cycle completed` |
| `component` | Service component | `polling`, `grpc`, `health`, `config` |
| `pair` | Currency pair (when applicable) | `RSD→EUR` |
| `provider` | Provider name (when applicable) | `fawazahmed0` |
| `duration_ms` | Operation duration in milliseconds | `142` |
| `error` | Error message (when applicable) | `HTTP 503 from provider` |

**Health Endpoint Response**

The HTTP health endpoint (`/readyz`) returns a JSON summary useful for operational monitoring:

```json
{
  "status": "healthy",
  "database": "connected",
  "active_pairs": 3,
  "outdated_pairs": 0,
  "providers": [
    {
      "name": "fawazahmed0",
      "status": "healthy",
      "consecutive_failures": 0,
      "last_success_at": "2026-03-21T16:00:00Z"
    },
    {
      "name": "exchangerate-api",
      "status": "degraded",
      "consecutive_failures": 2,
      "last_success_at": "2026-03-21T14:00:00Z"
    }
  ],
  "uptime_seconds": 86400
}
```

**Metrics Strategy (v1)**

In v1, operational metrics are derived from structured logs and the health endpoint — no dedicated metrics system (Prometheus, Grafana) is deployed. The metric definitions in Section 2.5.1 describe *what is tracked*; in v1 the data source is logs and the health endpoint JSON. A dedicated metrics exporter may be added in a future version when the ecosystem grows to warrant it.

---
<br />


# 4. Architecture Decision Records

## ADR-1: gRPC as Service API Protocol

**Status:** Accepted

**Context:** The Currency Rate Service needs an API protocol for inter-service communication. The Expense Tracker ecosystem already uses gRPC for its mobile client (Stage 6), and the team has established tooling (protoc, buf, grpcurl) and patterns (proto contracts, Go codegen) from that work. The primary consumer — Expense Tracker Backend — is a Go service capable of acting as a gRPC client.

**Decision:** Use gRPC with Protocol Buffers as the sole API protocol for rate data. HTTP is used only for the health check endpoint (`/healthz`, `/readyz`).

**Alternatives Considered:**

| Alternative | Reason for Rejection |
|-------------|---------------------|
| REST (JSON over HTTP) | No type-safe contract; requires manual serialization; adds OpenAPI/Swagger maintenance overhead; no ecosystem precedent for inter-service communication |
| GraphQL | Overengineered for the use case — the service has 4 fixed methods, not a flexible query surface; adds a runtime dependency with no benefit |
| Dual protocol (gRPC + REST gateway) | Unnecessary complexity — no browser or third-party consumer requires REST access in v1 |

**Rationale:**
- Proto file serves as a versioned API contract between services — breaking changes are detectable via `buf breaking`
- Go code generation eliminates manual serialization and reduces integration bugs
- Binary protocol is more efficient than JSON for the high-frequency polling data pattern
- Consistent with Expense Tracker's gRPC layer — shared tooling, familiar patterns, same proto build pipeline
- Aligns with the ecosystem's migration toward microservice architecture (BRD Section 7)

**Consequences:**
- Consumers must use a gRPC client (no browser-friendly API without a gateway)
- Proto file becomes a shared dependency — changes require coordination with consumers
- Debugging requires `grpcurl` or similar tooling (less convenient than `curl` for ad-hoc queries)

## ADR-2: Database-Driven Configuration vs Config File

**Status:** Accepted

**Context:** The service needs to store business configuration: currency pairs, provider definitions, polling intervals, and pair-provider assignments. Two approaches exist — static config files (YAML/TOML loaded at startup) or database tables read at runtime.

**Decision:** Store all business configuration in the database (`currency_pairs`, `providers`, `pair_provider_config` tables). Only system-level settings (ports, database DSN, log level) reside in environment variables. This satisfies BR-12.

**Alternatives Considered:**

| Alternative | Reason for Rejection |
|-------------|---------------------|
| Config file only (YAML/TOML) | Every change requires file edit + service restart; no runtime flexibility; hard to manage on remote servers |
| Config file with hot-reload (e.g., `fsnotify`) | Still requires file access on the server; adds filesystem watcher complexity; doesn't solve the "add a pair without SSH" problem |
| Hybrid: pairs in DB, providers in config | Inconsistent — provider URL changes would require restart while pair changes wouldn't; arbitrary split |

**Rationale:**
- Adding a new currency pair or swapping a provider is an INSERT/UPDATE — no SSH, no file edit, no restart
- Configuration is queryable and auditable — `SELECT * FROM currency_pairs` shows the full state
- The polling engine reloads config from DB on each tick cycle — changes take effect within one polling interval
- Consistent with the data model: pairs, providers, and assignments are relational data with foreign key dependencies — they belong in the database
- Only exception: custom adapter code requires deployment, but the *registration* of the custom provider is still a DB record

**Consequences:**
- Initial setup requires seed data migration (not just a config file copy)
- Configuration errors are harder to catch before they take effect — no static validation at startup (mitigated by CHECK constraints on tables)
- If the database is down, configuration cannot be reloaded — service continues with the last known configuration

## ADR-3: Hybrid Provider Adapter Architecture

**Status:** Accepted

**Context:** The service must integrate with multiple external rate APIs. Most providers (fawazahmed0, ExchangeRate-API, Frankfurter) follow the same pattern — HTTP GET to a URL, parse a JSON response, extract the rate value at a known path. However, future providers (crypto exchanges, WebSocket feeds, OAuth-protected APIs) may require custom logic. The architecture must support both cases without overengineering v1.

**Decision:** Implement a hybrid adapter system:
- **Generic JSON Adapter** — config-driven; handles any provider that returns a rate value in a standard REST JSON response. Configured via database fields: `base_url` (URL template), `rate_json_path` (path to rate value), `currency_code_mapping` (internal→provider code map). Adding a new standard provider requires only a database INSERT.
- **Custom Adapter Interface** — code-level; a Go interface (`RateProvider`) that any non-standard provider must implement. Registered by `adapter_type = 'custom'` in the `providers` table and resolved by provider `name` in the adapter registry.

**Alternatives Considered:**

| Alternative | Reason for Rejection |
|-------------|---------------------|
| Generic adapter only | Cannot handle non-standard sources (WebSocket, multi-step auth, response pagination) without compromising the generic config model |
| Custom adapter per provider | Requires code for every new provider, even simple REST JSON APIs — unnecessary development effort for standard cases |
| Plugin system (dynamic loading) | Overengineered for v1; Go plugin support is limited and fragile; adds build complexity |

**Rationale:**
- All three v1 providers (fawazahmed0, ExchangeRate-API, Frankfurter) are standard REST JSON — handled entirely by the generic adapter with zero code
- The custom interface exists as an extension point — not used in v1, but the architecture supports it without changes
- Provider-specific quirks (currency code casing, rate inversion for reverse pairs) are handled via configuration fields, not code
- See Section 5 for the full adapter specification, interface definition, and provider config examples

**Consequences:**
- Generic adapter must be flexible enough to handle URL templates, JSONPath variations, and code mappings — requires careful design of the config schema
- Custom adapters require code deployment — the only configuration operation that is not runtime-only
- The adapter registry must support both types transparently — the polling engine should not know which type it is calling

## ADR-4: Staleness Flag vs Rate Unavailable

**Status:** Accepted

**Context:** When a polling cycle fails for both primary and backup providers, the service has no fresh rate for the affected currency pair. Two approaches exist: return an error ("rate unavailable") or return the last known rate with a staleness indicator. This decision affects every consumer's error handling and UX.

**Decision:** Keep the last known rate and set `is_outdated = true`. The API always returns a rate value (if one was ever fetched) — the consumer decides how to handle the staleness. This satisfies BR-5.

**Alternatives Considered:**

| Alternative | Reason for Rejection |
|-------------|---------------------|
| Return `NOT_FOUND` error when rate is stale | Forces all consumers to implement fallback logic for missing rates; a 2-hour-old rate is more useful than no rate at all for informational display |
| Return rate without any staleness indicator | Consumer cannot distinguish a fresh rate from one that hasn't been updated in hours — silently stale data is worse than explicitly flagged data |
| Separate "confidence" field (e.g., 0.0–1.0) | Overengineered for v1 — binary fresh/outdated is sufficient; confidence scoring requires defining what score thresholds mean |

**Rationale:**
- The service is informational (not financial-grade) — an approximate rate from 2 hours ago is better than no rate for displaying currency equivalents in the Expense Tracker
- Different consumers may handle staleness differently: Expense Tracker might show a "≈" prefix, Invoice Generator might block conversion — the service should not impose a single policy
- The `is_outdated` flag is simple, unambiguous, and costs nothing to implement — it is a boolean column on the `rates` table
- Staleness is automatically cleared when the next successful poll inserts a fresh rate record (see Use Case 2.3.4)

**Consequences:**
- Consumers must check `is_outdated` and decide their own behavior — the service does not enforce a response to staleness
- A rate could theoretically be days old and still returned — consumers should consider `updated_at` alongside `is_outdated` for time-sensitive decisions
- The approach requires clear API documentation so consumers understand the staleness contract

## ADR-5: Single Table for Current and Historical Rates

**Status:** Accepted

**Context:** The service needs to store both the current rate (for `GetRate`) and historical rates (for `GetRateHistory`). Two schema approaches exist: a single `rates` table where "current" is the most recent record per pair, or two separate tables — `current_rates` (one row per pair, updated in place) and `rate_history` (append-only log).

**Decision:** Use a single `rates` table. Every successful poll inserts a new row. The "current rate" is the most recent record for a pair, resolved by `ORDER BY fetched_at DESC LIMIT 1` on the `rates_latest_idx` index.

**Alternatives Considered:**

| Alternative | Reason for Rejection |
|-------------|---------------------|
| Two tables: `current_rates` + `rate_history` | Data duplication — every poll writes to both tables; `current_rates` UPDATE + `rate_history` INSERT adds transactional complexity; staleness flag must be synchronized across two tables |
| Materialized view for current rates | Adds refresh overhead and PostgreSQL-specific complexity; unnecessary for the expected query volume |
| In-memory cache for current rates | Loses data on restart; adds cache invalidation complexity; DB query at <50ms is fast enough |

**Rationale:**
- Simpler schema — one table, one write path, one source of truth
- No data duplication or synchronization between current and historical stores
- The `rates_latest_idx` index on `(currency_pair_id, fetched_at DESC)` makes the "current rate" query an efficient index scan — no full table scan regardless of table size
- Historical queries use `rates_history_idx` on `(currency_pair_id, fetched_at ASC)` for range scans
- At hourly polling for 3 pairs, the table grows by ~26K rows/year — trivial for PostgreSQL, no partitioning or archival needed in v1

**Consequences:**
- "Get current rate" query is slightly more complex than a simple primary key lookup on a dedicated `current_rates` table — mitigated by the covering index
- Table will grow indefinitely in v1 — future versions may introduce a retention policy (e.g., aggregate to daily averages after 1 year, or partition by month)
- Staleness flag (`is_outdated`) is set on the latest record via UPDATE — the only UPDATE operation on this otherwise append-only table

## ADR-6: Plaintext gRPC for Demo

**Status:** Accepted

**Context:** The demo environment runs on a Hetzner VPS behind Cloudflare. The Expense Tracker already uses plaintext gRPC on port `50051` with direct IP access because Cloudflare's free plan does not support gRPC proxying (HTTP/2 with gRPC content-type). The Currency Rate Service needs the same connectivity for its demo deployment.

**Decision:** Use plaintext (no TLS) gRPC for both development and demo environments. TLS will be added when a production deployment is planned.

**Alternatives Considered:**

| Alternative | Reason for Rejection |
|-------------|---------------------|
| Self-signed TLS certificates | Adds certificate management overhead; consumers (Expense Tracker) would need to trust the self-signed CA; no security benefit for a demo with public data |
| Let's Encrypt TLS for gRPC | Requires domain-based routing which Cloudflare free plan doesn't support for gRPC; direct IP + Let's Encrypt requires custom ACME setup |
| Cloudflare Tunnel for gRPC | Free plan limitation — gRPC reflection and streaming do not work through Cloudflare Tunnel (confirmed in Expense Tracker Stage 6) |

**Rationale:**
- Consistent with Expense Tracker's gRPC deployment (ADR-4 in Expense Tracker TDD) — same infrastructure, same constraints, same solution
- The service handles public exchange rate data — no authentication, no sensitive information in transit
- Demo environment is for portfolio demonstration, not production use — plaintext is acceptable
- Direct IP access on port `50052` is sufficient for inter-service communication on the same host and for external `grpcurl` testing

**Consequences:**
- gRPC traffic is unencrypted — acceptable for public rate data, not suitable for production with sensitive payloads
- Access is via direct IP only (`46.224.29.194:50052`) — no domain name or Cloudflare protection for gRPC
- When production TLS is needed, the transition requires: certificate provisioning, gRPC server TLS config, consumer client TLS config — a coordinated change across services

---
<br />


# 5. Provider Adapter Specification

## 5.1 Provider Interface

Every provider adapter — whether generic (config-driven) or custom (code) — must implement the same contract. The polling engine interacts with adapters through the adapter registry and does not distinguish between adapter types.

**Adapter Contract**

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| FetchRate | Context (with timeout), CurrencyPair | RateResult or error | Retrieves the current exchange rate for a pair from the external provider. Must respect the context deadline (`PROVIDER_HTTP_TIMEOUT`). |
| Name | — | string | Returns the provider's unique identifier, matching `providers.name` in the database. |

**CurrencyPair (input)**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| ID | integer | Primary key from `currency_pairs` table | `1` |
| FromCurrency | string | Source currency (ISO 4217) | `RSD` |
| ToCurrency | string | Target currency (ISO 4217) | `EUR` |

**RateResult (output on success)**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| Rate | decimal | Exchange rate value | `0.00853` |
| FetchedAt | timestamp | When the rate was retrieved from the provider | `2026-03-21T16:00:00Z` |
| SourceName | string | Provider name for attribution | `fawazahmed0` |

**Error Handling Contract**

| Error Type | Cause | Handling |
|-----------|-------|---------|
| HTTP error (4xx, 5xx) | Provider endpoint returned an error status | Return wrapped error; adapter registry triggers failover |
| Timeout | Provider did not respond within `PROVIDER_HTTP_TIMEOUT` | Context deadline exceeded; return error; failover triggered |
| Parse error | Response JSON is malformed or rate value not found at expected path | Return error with details; failover triggered |
| Rate value invalid | Parsed rate is zero, negative, or non-numeric | Return validation error; failover triggered |

All errors returned by `FetchRate` are treated equally by the adapter registry — any error triggers failover to the backup provider (if configured).

## 5.2 Generic JSON Adapter

The generic JSON adapter handles any provider that follows the pattern: HTTP GET to a URL → JSON response → extract rate value at a known path. All v1 providers use this adapter.

**Configuration Schema (from `providers` table)**

| Field | Purpose | Example (fawazahmed0) |
|-------|---------|----------------------|
| `base_url` | URL template with `{from}` and `{to}` placeholders | `https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/{from}.min.json` |
| `rate_json_path` | Dot-notation path to the rate value in JSON response | `{from}.{to}` |
| `currency_code_mapping` | JSONB map: internal code → provider code | `{"RSD": "rsd", "EUR": "eur", "USD": "usd"}` |
| `api_key` | API key appended as query parameter (if required) | `null` (not needed) |

**URL Template Variables**

| Variable | Replaced With | Source |
|----------|---------------|--------|
| `{from}` | Source currency code (after mapping) | `currency_code_mapping[pair.FromCurrency]` or `pair.FromCurrency` if no mapping |
| `{to}` | Target currency code (after mapping) | `currency_code_mapping[pair.ToCurrency]` or `pair.ToCurrency` if no mapping |

**Fetch Algorithm**

1. Resolve provider-specific currency codes via `currency_code_mapping` (fallback: use internal codes as-is)
2. Substitute `{from}` and `{to}` in `base_url` to produce the request URL
3. Execute HTTP GET with context timeout (`PROVIDER_HTTP_TIMEOUT`)
4. Parse JSON response body
5. Navigate to the rate value using `rate_json_path` (with `{from}` / `{to}` substituted)
6. Validate: rate must be a positive number
7. Return the result: rate value, current timestamp as `FetchedAt`, provider name as `SourceName`

**Rate Inversion**

Some providers may only return rates in one direction (e.g., EUR→RSD but not RSD→EUR). When the `pair_provider_config` maps a pair to a provider that returns the inverse rate, the adapter supports automatic inversion:

- If the direct URL returns a rate, use it as-is
- If the pair requires inversion (future configuration flag on `pair_provider_config`), compute `1 / rate`
- v1 simplification: fawazahmed0 returns all directions natively, so inversion is not needed for the initial provider set. The mechanism is documented for future use.

**JSONPath Examples by Provider**

**fawazahmed0**

```
URL:      https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/{from}.min.json
Path:     {from}.{to}
Mapping:  {"RSD": "rsd", "EUR": "eur", "USD": "usd"}

Example request:  GET .../currencies/rsd.min.json
Example response: {"date":"2026-03-21", "rsd":{"eur":0.00853, "usd":0.00928, ...}}
Path resolved:    rsd.eur → 0.00853
```

Fallback URL (if CDN is unavailable): `https://latest.currency-api.pages.dev/v1/currencies/{from}.min.json`

**ExchangeRate-API (Open Access)**

```
URL:      https://open.er-api.com/v6/latest/{from}
Path:     rates.{to}
Mapping:  null (uses uppercase ISO codes natively)

Example request:  GET .../latest/RSD
Example response: {"result":"success", "base_code":"RSD", "rates":{"EUR":0.00853, "USD":0.00928, ...}}
Path resolved:    rates.EUR → 0.00853
```

> Note: ExchangeRate-API open access endpoint recommends no more than one request per day. Suitable as a backup provider with daily polling or as cross-validation source.

**Frankfurter**

```
URL:      https://api.frankfurter.dev/v1/latest?base={from}&symbols={to}
Path:     rates.{to}
Mapping:  null (uses uppercase ISO codes natively)

Example request:  GET .../latest?base=EUR&symbols=USD
Example response: {"base":"EUR", "date":"2026-03-21", "rates":{"USD":1.0842}}
Path resolved:    rates.USD → 1.0842
```

> **Limitation:** Frankfurter uses European Central Bank data and does **not support RSD**. It can only serve the EUR↔USD pair in v1. Not suitable as a provider for RSD pairs.

## 5.3 Custom Adapter Interface

**When to Use**

A custom adapter is required when a provider does not fit the generic JSON adapter pattern:

| Scenario | Example |
|----------|---------|
| Non-standard response format | XML, CSV, or nested/paginated JSON requiring multi-step extraction |
| Authentication flow | OAuth2 token exchange, HMAC-signed requests |
| Multi-endpoint composition | Rate requires data from two API calls (e.g., bid/ask averaging) |
| WebSocket or streaming source | Real-time crypto exchange feeds |
| Rate calculation | Derived rate from intermediate currencies (e.g., RSD→BTC via RSD→USD→BTC) |

**Implementation Contract**

1. Implement the adapter contract defined in Section 5.1 — both methods (`FetchRate` and `Name`)
2. Register the adapter in the adapter registry at startup (code-level registration by provider name)
3. Set `adapter_type = 'custom'` in the `providers` table record

**Registration**

Custom adapters are registered at application startup by provider name. The adapter registry resolves providers by `adapter_type`:

| adapter_type | Resolution |
|-------------|------------|
| `generic_json` | Instantiated automatically from the provider's DB configuration (URL template, JSONPath, code mapping) |
| `custom` | Looked up by `providers.name` from the pre-registered adapter map; requires code deployment |

**v1 Scope**

No custom adapters are needed in v1 — all three providers (fawazahmed0, ExchangeRate-API, Frankfurter) are standard REST JSON and are handled by the generic adapter. The custom adapter interface exists as an extension point for future crypto or non-standard providers.

## 5.4 Provider Configuration Examples

**v1 Seed Data: `providers` Table**

| name | adapter_type | base_url | rate_json_path | currency_code_mapping | api_key | is_active |
|------|-------------|----------|----------------|----------------------|---------|-----------|
| fawazahmed0 | generic_json | `https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/{from}.min.json` | `{from}.{to}` | `{"RSD":"rsd","EUR":"eur","USD":"usd"}` | null | true |
| fawazahmed0-fallback | generic_json | `https://latest.currency-api.pages.dev/v1/currencies/{from}.min.json` | `{from}.{to}` | `{"RSD":"rsd","EUR":"eur","USD":"usd"}` | null | true |
| exchangerate-api | generic_json | `https://open.er-api.com/v6/latest/{from}` | `rates.{to}` | null | null | true |
| frankfurter | generic_json | `https://api.frankfurter.dev/v1/latest?base={from}&symbols={to}` | `rates.{to}` | null | null | true |

> `fawazahmed0-fallback` is registered as a separate provider (not a built-in fallback mechanism) because it has a different base URL. This allows the pair-provider assignment to use it as a backup independently.

**v1 Seed Data: `pair_provider_config` Table**

| currency_pair | provider | priority | is_active | Rationale |
|---------------|----------|----------|-----------|-----------|
| RSD→EUR | fawazahmed0 | primary | true | Supports RSD, no rate limits, reliable CDN |
| RSD→EUR | fawazahmed0-fallback | backup | true | Same data, alternative endpoint |
| RSD→USD | fawazahmed0 | primary | true | Supports RSD, no rate limits |
| RSD→USD | exchangerate-api | backup | true | Supports RSD, free open access |
| EUR→USD | frankfurter | primary | true | ECB data source, high quality for EUR pairs |
| EUR→USD | fawazahmed0 | backup | true | Universal fallback with broad currency support |

**Provider Coverage Matrix**

| Provider | RSD↔EUR | RSD↔USD | EUR↔USD | Rate Limits | Auth |
|----------|---------|---------|---------|-------------|------|
| fawazahmed0 (CDN) | ✅ | ✅ | ✅ | None | None |
| fawazahmed0 (pages.dev) | ✅ | ✅ | ✅ | None | None |
| ExchangeRate-API (open) | ✅ | ✅ | ✅ | ~1/day recommended | None |
| Frankfurter | ❌ | ❌ | ✅ | None | None |

> **Key constraint:** Frankfurter does not support RSD (Serbian Dinar is not published by the ECB). It is only assigned to the EUR↔USD pair where ECB data is the highest quality source.

---