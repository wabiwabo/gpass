# Phase 7: Observability & Monitoring — Design Specification

**Version:** 1.0  
**Date:** 2026-04-06  
**Status:** Draft  
**Depends on:** Phase 1-6, Shared Go Library (golib)

---

## 1. Overview

Phase 7 adds enterprise-grade observability: OpenTelemetry tracing, Prometheus metrics, structured audit logging, and Grafana dashboards. Every request is traceable end-to-end across all services.

### 1.1 Scope

**In scope:**
- OpenTelemetry SDK integration for distributed tracing (OTLP export)
- Prometheus metrics endpoint per service (/metrics)
- Standard metrics: request count, latency histogram, error rate, active connections
- Business metrics: verifications, signings, API key validations, consent grants
- Jaeger for trace visualization (dev/staging)
- Grafana + Prometheus for metrics dashboards
- Alertmanager rules for SLO-based alerts
- Health check aggregation dashboard

**Out of scope:**
- Log aggregation (Loki/ELK) → future
- Custom APM → future
- Real User Monitoring (RUM) → future

### 1.2 Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Tracing | OpenTelemetry SDK (Go) | Vendor-neutral, industry standard |
| Trace backend | Jaeger (dev), OTLP collector (prod) | Jaeger is free, OTLP is portable |
| Metrics | Prometheus client_golang | De facto standard for Go services |
| Dashboards | Grafana | Universal, free, supports Prometheus |
| Alerts | Alertmanager | Native Prometheus integration |

---

## 2. Architecture

```
Services → OpenTelemetry SDK → OTLP Collector → Jaeger (traces)
                              → Prometheus (scrape /metrics)
                              → Grafana (visualize)
                              → Alertmanager (alert)
```

### 2.1 Metrics Stack (Docker Compose additions)

```yaml
  prometheus:
    image: prom/prometheus:v2.51.0
    ports: ["9090:9090"]
    volumes:
      - ./infrastructure/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:11.4.0
    ports: ["3002:3000"]
    volumes:
      - ./infrastructure/grafana/provisioning:/etc/grafana/provisioning

  jaeger:
    image: jaegertracing/all-in-one:1.54
    ports:
      - "16686:16686"   # UI
      - "4317:4317"     # OTLP gRPC
      - "4318:4318"     # OTLP HTTP
```

### 2.2 Prometheus Scrape Config

```yaml
# infrastructure/prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "garudapass-services"
    static_configs:
      - targets:
          - "bff:4000"
          - "identity:4001"
          - "garudainfo:4003"
          - "garudacorp:4006"
          - "garudasign:4007"
          - "garudaportal:4009"
    metrics_path: /metrics
```

---

## 3. Metrics per Service

### 3.1 Standard HTTP Metrics (all services)

```
http_requests_total{service, method, path, status_code}        — counter
http_request_duration_seconds{service, method, path}           — histogram
http_requests_in_flight{service}                                — gauge
http_request_size_bytes{service, method, path}                 — histogram
http_response_size_bytes{service, method, path}                — histogram
```

### 3.2 Business Metrics

```
# Identity
identity_verifications_total{result}                            — counter (success/failure)
identity_otp_sent_total{channel}                                — counter (sms/email)

# GarudaInfo
consent_grants_total{purpose}                                   — counter
consent_revocations_total                                       — counter

# GarudaCorp
entity_registrations_total{entity_type}                         — counter
role_assignments_total{role}                                    — counter

# GarudaSign
certificates_issued_total                                       — counter
documents_signed_total{pades_level}                             — counter
signing_duration_seconds                                        — histogram

# GarudaPortal
api_key_validations_total{result}                               — counter (valid/invalid/rate_limited)
webhook_deliveries_total{status}                                — counter (delivered/failed)
api_usage_daily{app_id, tier}                                   — gauge
```

### 3.3 Infrastructure Metrics

```
# Circuit breaker
circuit_breaker_state{service, target}                          — gauge (0=closed, 1=open, 2=half-open)
circuit_breaker_failures_total{service, target}                 — counter

# Database (when PostgreSQL adapter added)
db_connections_active{service}                                  — gauge
db_query_duration_seconds{service, query}                       — histogram
```

---

## 4. Tracing

### 4.1 Trace Propagation

All services propagate W3C Trace Context (traceparent header) through the BFF to internal services. Each service creates spans for:
- HTTP handler (automatic via middleware)
- External HTTP calls (circuit breaker client)
- Database queries (future)

### 4.2 Span Attributes

```
service.name        — e.g. "garudasign"
http.method          — GET, POST, etc.
http.url             — request URL
http.status_code     — response status
user.id              — from X-User-ID header
request.id           — from X-Request-Id header
```

---

## 5. Implementation in golib

### 5.1 New packages in packages/golib/

```
packages/golib/
├── metrics/
│   ├── prometheus.go          # Prometheus metrics registration
│   ├── prometheus_test.go
│   ├── middleware.go           # HTTP metrics middleware
│   └── middleware_test.go
├── tracing/
│   ├── otel.go                # OpenTelemetry tracer initialization
│   ├── otel_test.go
│   ├── middleware.go           # HTTP tracing middleware
│   └── middleware_test.go
```

### 5.2 Metrics Middleware

```go
// Usage in service main.go:
metricsMiddleware := metrics.NewHTTPMiddleware("garudasign")
mux.Handle("/metrics", metrics.Handler())
// Wrap all routes: metricsMiddleware(mux)
```

### 5.3 Tracing Middleware

```go
// Usage in service main.go:
shutdown := tracing.Init("garudasign", tracing.WithOTLPEndpoint("jaeger:4317"))
defer shutdown()
// Wrap: tracing.Middleware(mux)
```

---

## 6. Grafana Dashboards

### 6.1 Platform Overview Dashboard

Cards: Total requests/min, Error rate %, P99 latency, Active services

Panels:
- Request rate by service (time series)
- Error rate by service (time series)
- Latency P50/P95/P99 by service (time series)
- Service health status (stat panel)

### 6.2 Business Metrics Dashboard

Cards: Verifications today, Signings today, API calls today, Active developers

Panels:
- Identity verifications over time
- Documents signed over time
- API key validations by result
- Webhook delivery success rate

---

## 7. Alerting Rules

```yaml
# infrastructure/prometheus/alerts.yml
groups:
  - name: garudapass-slo
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status_code=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Error rate > 1% for {{ $labels.service }}"

      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "P99 latency > 2s for {{ $labels.service }}"

      - alert: CircuitBreakerOpen
        expr: circuit_breaker_state > 0
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Circuit breaker open: {{ $labels.service }} → {{ $labels.target }}"
```
