# Database Migrations and Schema Evolution Guide

IvyTicketing manages schema changes using [Goose](https://github.com/pressly/goose). All migrations reside in [`database/migrations/`](file:///root/ivyticketing/database/migrations/).

---

## 1. Migration File Structure

Every migration follows a 5-digit sequential prefix with explicit up and down SQL blocks:

```sql
-- +goose Up
-- SQL statements applied during forward migration
CREATE TABLE example_table (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
-- SQL statements applied during rollback
DROP TABLE IF EXISTS example_table;
```

---

## 2. Executing Migrations

### Running Migrations Up (Local / CI)
```bash
goose -dir database/migrations postgres "$DATABASE_URL" up
```

### Checking Migration Status
```bash
goose -dir database/migrations postgres "$DATABASE_URL" status
```

### Rolling Back the Most Recent Migration
```bash
goose -dir database/migrations postgres "$DATABASE_URL" down
```

---

## 3. Zero-Downtime Production Guidelines

1. **Avoid Destructive Table Rewrites**: Never rename columns or drop columns actively referenced by running backend API servers. Use expand-and-contract patterns across two deployment cycles.
2. **Concurrent Indexing**: In production databases under heavy load, create indexes concurrently to prevent table lockouts:
   ```sql
   -- +goose Up
   -- +goose NO TRANSACTION
   CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_orders_event_status ON orders (event_id, status);
   ```
3. **Safe Column Additions**: When adding new columns, always provide a default value or allow `NULL` so existing insert queries do not fail before application code is updated.
