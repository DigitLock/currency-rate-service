# Currency Rate Service - BRD Amendment 1

# 1. Document Overview
- **Document Owner:** Igor Kudinov — Business & System Analyst
- **Date:** 2026-10-05
- **Amends:** Currency Rate Service BRD v1.0. Sections of the BRD not mentioned here stay in force.

---

# 2. Business Context

A new consumer, **Crypto Account Service (CAS)**, depends on the Currency Rate Service. Its milestone S2 (`card-auth`) converts a card authorization amount into a token amount using a CRS rate.

- CAS calls `GetRate(from = authorization currency, to = USD)` over gRPC within a 2.5 s authorization decision deadline
- USD needs no rate; EUR and RSD use the existing EUR→USD and RSD→USD pairs; GBP and CHF need new pairs; any other currency is declined by CAS
- CAS does money arithmetic in decimals and declines when `is_outdated` is true

CAS's own rules live in CAS documentation.

---

# 3. Stakeholders

One row is added to BRD Section 5 under **System Consumers**:

| Name | Department / Role | Responsibility / Interest | Influence |
|------|--------------------|---------------------------|------------|
| Crypto Account Service | Consumer (gRPC client) | Uses USD rates for card authorization quotes; depends on GBP→USD, CHF→USD and an exact decimal rate | High |

---

# 4. Requirement Changes

| BR | Change | Acceptance |
|----|--------|------------|
| BR-7 Configurable Currency Pairs | Scope extended to five pairs, each served only in its stored direction: RSD→EUR, RSD→USD, EUR→USD, GBP→USD, CHF→USD | `ListSupportedPairs` returns five pairs; `GetRate` returns a fresh rate for GBP→USD and CHF→USD from the primary provider, and from the backup when the primary is disabled |
| BR-4 gRPC Rate API | The `Rate` message also carries the exact stored rate as a decimal string (`rate_decimal`); the existing double field is unchanged | `GetRate` and `GetRates` return both; the string equals the stored value digit for digit; existing clients are unaffected |
| BR-7 / BR-12 "no restart" | Clarification: v1 read pairs and providers only at startup, which did not meet the stated acceptance. Configuration changes now take effect within a reload interval (`CONFIG_RELOAD_INTERVAL`, default 60 s) without restart | A pair or provider mapping added to the database on a running service starts being polled within the reload interval |

---

# 5. Rate Age

**Decision of 2026-10-05:** all configured providers publish one rate per day, so a shorter polling interval would not give fresher rates.

- Consumers receive a daily rate
- `is_outdated` shows a failed polling cycle, not the age of the provider's rate
- An intraday source is in the README backlog

---

# 6. Out of Scope of This Amendment

- Shorter polling interval
- Crypto pairs (a separate later amendment)
- Docker image and deployment
- The general testing phase of the roadmap
