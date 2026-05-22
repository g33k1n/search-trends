# Search Trends

Backend service for the "Now searching" widget. It consumes search events from NATS and exposes the top search queries for the last 5 minutes over HTTP.

## Quick Start

```bash
docker compose up --build
```

Health check:

```bash
curl http://localhost:8080/health
```

Publish a few events:

```bash
docker compose exec nats-box nats pub --server nats://nats:4222 search.events '{"query":"iphone 15","user_id":"u1","request_id":"r1","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","source":"search-api"}'
docker compose exec nats-box nats pub --server nats://nats:4222 search.events '{"query":"iphone 15","user_id":"u2","request_id":"r2","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","source":"search-api"}'
docker compose exec nats-box nats pub --server nats://nats:4222 search.events '{"query":"sneakers","user_id":"u3","request_id":"r3","timestamp":"'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'","source":"search-api"}'
```

Get top queries:

```bash
curl 'http://localhost:8080/top?limit=10'
```

Manage the stop-list without restarting the service:

```bash
curl -X POST http://localhost:8080/stop-list \
  -H 'Content-Type: application/json' \
  -d '{"query":"iphone 15"}'

curl http://localhost:8080/stop-list
curl -X DELETE http://localhost:8080/stop-list/iphone%2015
```

Prometheus-style metrics:

```bash
curl http://localhost:8080/metrics
```

## Event Contract

The service expects JSON messages in the NATS subject `search.events`.

```json
{
  "query": "iphone 15",
  "user_id": "u123",
  "request_id": "req-456",
  "timestamp": "2026-05-22T12:00:00Z",
  "source": "search-api"
}
```

Required field:

- `query`: raw user search query. The service normalizes it by trimming spaces, lowercasing and collapsing whitespace.

Useful fields for production evolution:

- `timestamp`: event time. It allows the service to handle minor delivery delays and still count the event in the correct bucket.
- `user_id`: optional stable user identifier. This is not used in the current implementation, but it is needed for stronger anti-abuse logic such as per-user rate limits.
- `request_id`: optional idempotency and tracing key.
- `source`: producer name, useful for debugging and source-level metrics.

## Architecture

The current storage is in-memory because the widget needs very fast reads and the task does not require historical analytics. The core data structure is a sliding time window split into small buckets. By default it keeps 5 minutes with 1-second resolution.

Each bucket stores counters by normalized query. The service also keeps total counters for the whole active window. When a bucket expires or is reused, its counts are subtracted from the totals. This makes ingestion cheap and avoids scanning all events.

Reads are expected to be 10-50 times more frequent than writes. For that reason `/top` returns a cached sorted snapshot. The snapshot is rebuilt only after writes, bucket expiration, or stop-list updates.

To reduce the effect of obvious artificial spikes, the service caps the number of equal normalized queries accepted into one bucket. The default is `1000` per second and can be changed with `MAX_QUERY_PER_BUCKET`.

The stop-list is applied dynamically. Blocked queries are hidden from the top immediately, and newly received blocked queries are rejected. Historical counts are kept in memory, so if a word is removed from the stop-list while it is still inside the 5-minute window, it can appear again.

## Trade-offs

- The service is intentionally in-memory. Restarting it loses the current 5-minute window. For this widget that can be acceptable; if not, a compact replicated store or changelog replay should be added.
- Sorting all unique queries on snapshot rebuild is simple and predictable. If cardinality becomes very high, this can be replaced with a heap or approximate heavy-hitters algorithm.
- The anti-abuse cap protects the widget from extreme single-query bursts, but it can undercount a real viral query during peak traffic. A production version should combine this with per-user, per-session or per-fingerprint limits.
- Event timestamps older than the active window are ignored, and timestamps too far in the future are counted as current time.

## Tests

```bash
go test ./...
```

## Benchmarks

A quick read-load check can be run after starting the stack:

```bash
hey -z 30s -c 100 'http://localhost:8080/top?limit=10'
```

For Go-level benchmarks, add targeted benchmarks around `internal/trends.Store` with the expected unique-query cardinality.
