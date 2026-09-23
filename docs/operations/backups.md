# Database Backup and Disaster Recovery Procedures

This guide provides procedures for backing up PostgreSQL 16 and Redis 7, verifying backup integrity, and executing Point-in-Time Recovery (PITR).

---

## 1. PostgreSQL Backup Strategies

IvyTicketing requires two backup tiers: daily automated logical snapshots and continuous write-ahead log (WAL) archiving.

### 1. Daily Logical Snapshot (`pg_dump`)
Execute daily compressed snapshots using PostgreSQL custom archive format:
```bash
docker exec -t ivyticketing-pg pg_dump -U postgres -d ivyticketing -Fc -f /backups/ivyticketing_$(date +%Y%m%d_%H%M%S).dump
```

### 2. Physical WAL Archiving (PITR)
For enterprise zero-RPO configurations, configure PostgreSQL `archive_mode` to push WAL segments to encrypted S3 storage:
```ini
# postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'aws s3 cp %p s3://ivyticketing-wal-archive/%f'
archive_timeout = 300
```

---

## 2. Restoring PostgreSQL from Backup

### Restoring to a Fresh Database Instance
1. Create a target database:
   ```bash
   createdb -U postgres ivyticketing_restored
   ```
2. Restore using `pg_restore` with parallel worker jobs:
   ```bash
   pg_restore -U postgres -d ivyticketing_restored -j 4 --clean --if-exists /backups/ivyticketing_20260923.dump
   ```
3. Verify table counts and row integrity:
   ```sql
   SELECT count(*) FROM orders;
   SELECT count(*) FROM tickets;
   SELECT count(*) FROM users;
   ```

---

## 3. Redis Persistence Settings

While PostgreSQL is the permanent system of record, Redis state can be persisted to disk to avoid cold queue restarts:

```ini
# redis.conf
# Enable Append Only File (AOF) with 1-second sync
appendonly yes
appendfsync everysec

# Take snapshot if 10,000 keys change in 5 minutes
save 300 10000
```
