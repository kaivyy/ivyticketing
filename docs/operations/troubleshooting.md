# Operational Troubleshooting and Incident Diagnostics

This guide provides diagnostic procedures and SQL queries for troubleshooting common production incidents in IvyTicketing.

---

## 1. Participant Paid but Ticket Not Issued

### Symptom
Participant was charged by their bank/e-wallet, but their order remains `PENDING_PAYMENT` and no digital ticket appears under **My Tickets**.

### Root Cause
Payment gateway webhook was dropped, experienced a network timeout, or signature verification failed.

### Diagnostic SQL Queries
1. Check the order status and gateway reference:
   ```sql
   SELECT id, order_number, status, total, created_at, expires_at
   FROM orders
   WHERE order_number = 'ORD-20260923-XXXX'
      OR participant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
   ```
2. Search inbound webhook delivery attempts:
   ```sql
   SELECT id, gateway, status, error, created_at, payload
   FROM payment_webhooks
   WHERE payload::text LIKE '%ORD-20260923-XXXX%'
   ORDER BY created_at DESC;
   ```
3. If the webhook payload is present but failed processing, inspect the `error` column.

### Manual Resolution
If payment was verified externally in the gateway dashboard:
```sql
BEGIN;
UPDATE orders
SET status = 'PAID', paid_at = NOW()
WHERE id = :order_id AND status = 'PENDING_PAYMENT';

INSERT INTO tickets (order_id, event_id, category_id, participant_id, status)
SELECT o.id, o.event_id, i.category_id, o.participant_id, 'VALID'
FROM orders o
JOIN order_items i ON o.id = i.order_id
WHERE o.id = :order_id;

DELETE FROM inventory_reservations WHERE order_id = :order_id;
COMMIT;
```

---

## 2. Quota Discrepancy (Ghost Reservations)

### Symptom
Category indicates "Sold Out", but the number of confirmed paid tickets is significantly less than the total quota.

### Root Cause
Uncompleted checkout orders did not have their reservations purged due to a paused or lagging worker.

### Diagnostic SQL Queries
1. Compare category quota balances:
   ```sql
   SELECT id, name, total_quota, available_quota, reserved_quota, sold_quota
   FROM categories
   WHERE event_id = :event_id;
   ```
2. Find expired inventory holds that have not been cleaned up:
   ```sql
   SELECT r.id, r.order_id, r.quantity, r.expires_at, o.status
   FROM inventory_reservations r
   JOIN orders o ON r.order_id = o.id
   WHERE r.expires_at < NOW()
     AND o.status = 'PENDING_PAYMENT';
   ```

### Resolution
Run the manual cleanup sweep query or restart `cmd/worker`:
```sql
BEGIN;
UPDATE categories c
SET available_quota = c.available_quota + r.quantity,
    reserved_quota = c.reserved_quota - r.quantity
FROM inventory_reservations r
JOIN orders o ON r.order_id = o.id
WHERE c.id = o.category_id
  AND o.status = 'PENDING_PAYMENT'
  AND r.expires_at < NOW();

UPDATE orders
SET status = 'EXPIRED'
WHERE status = 'PENDING_PAYMENT'
  AND expires_at < NOW();
COMMIT;
```

---

## 3. Investigating High Database Connection Saturation

### Symptom
API latency spikes, Prometheus alerts trigger on `db_connections_acquired > 85%`.

### Diagnostic SQL Query
Find active executing queries and connection states:
```sql
SELECT pid, usename, client_addr, state, wait_event_type, wait_event, query_start, now() - query_start AS duration, query
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY duration DESC;
```

### Resolution
1. Look for long-running locks or blocked `SELECT ... FOR UPDATE` transactions.
2. If a specific query is stalled, cancel it gracefully:
   ```sql
   SELECT pg_cancel_backend(:pid);
   ```
3. If unresponsive, terminate the connection:
   ```sql
   SELECT pg_terminate_backend(:pid);
   ```
