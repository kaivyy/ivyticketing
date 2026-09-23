# Diagnostics, Profiling, and Trace Inspection

This guide details tools and techniques for diagnosing performance bottlenecks, profiling memory and CPU utilization, and inspecting distributed traces in IvyTicketing.

---

## 1. Structured Logging and Request Tracing

IvyTicketing utilizes Go's native `log/slog` structured logging package:

- **Request ID Tracing**: Every inbound HTTP request is assigned a unique `X-Request-Id` header by [`middleware.RequestID`](file:///root/ivyticketing/services/api/internal/platform/middleware/request_id.go).
- **Log Correlation**: The request ID is passed through context and included in all log output:
  ```json
  {"time":"2026-09-23T14:30:15Z","level":"INFO","msg":"order created","request_id":"req_01HZX876ABCXYZ","order_id":"e4eebc99-9c0b...","total":850000}
  ```
- **Searching Logs by Request**:
  ```bash
  grep "req_01HZX876ABCXYZ" /var/log/ivyticketing/api.log
  ```

---

## 2. Profiling with Go pprof

To diagnose CPU spikes or memory leaks under heavy load:

1. Enable pprof endpoints in development or staging:
   ```go
   import _ "net/http/pprof"
   ```
2. Capture a 30-second CPU profile during a simulated war queue surge:
   ```bash
   go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
   ```
3. Inspect memory allocations (heap profile):
   ```bash
   go tool pprof http://localhost:8080/debug/pprof/heap
   ```
4. Generate interactive visual graph:
   ```bash
   go tool pprof -http=:8081 profile.pb.gz
   ```

---

## 3. Query Analysis with PostgreSQL `EXPLAIN`

To analyze database query execution plans and optimize slow queries:

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT o.id, o.order_number, o.status, i.category_id
FROM orders o
JOIN order_items i ON o.id = i.order_id
WHERE o.event_id = 'b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22'
  AND o.status = 'PAID';
```
Look for:
- Sequential table scans (`Seq Scan`) on large tables.
- High shared buffer reads.
- Missing indexes on foreign keys.

---

## 4. Redis Inspection

Inspect live waiting queues and cache keys:
```bash
# Count waiting participants for an event
redis-cli ZCARD queue:b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22:waiting

# View top 5 waiting members
redis-cli ZRANGE queue:b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22:waiting 0 4 WITHSCORES

# Check cached status for a participant
redis-cli GET queue:status:b1eebc99-9c0b-4ef8-bb6d-6bb9bd380a22:a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11
```
