# Runbook: Database Connection Issues

**Alert:** DatabaseConnectionFailure / DatabaseConnectionsExhausted / SlowDatabaseQueries  
**Severity:** Critical / High  
**Service:** database  

---

## Summary

This alert fires when the application cannot connect to the database or database performance is degraded.

---

## Immediate Assessment (1 minute)

### 1. Check Database Process
```bash
# Is PostgreSQL running?
systemctl status postgresql
ps aux | grep postgres

# Check SQLite (if using)
ls -la /path/to/catalogizer.db
```

### 2. Test Connectivity
```bash
# PostgreSQL
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT 1;"

# Test from application host
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT version();"
```

### 3. Check Connection Count
```sql
-- PostgreSQL
SELECT count(*) FROM pg_stat_activity;
SELECT max_connections FROM pg_settings;
```

---

## Diagnosis Steps

### Scenario A: Database Server Down

```bash
# Check if PostgreSQL process exists
pgrep -f postgres

# Check logs
journalctl -u postgresql -n 50
tail -50 /var/log/postgresql/postgresql-*.log
```

**Common Causes:**
- Server crash
- Out of memory (OOM killer)
- Disk space full
- Configuration error

### Scenario B: Connection Pool Exhausted

```sql
-- Check active connections
SELECT 
    state,
    count(*)
FROM pg_stat_activity
GROUP BY state;

-- Check idle connections
SELECT pid, usename, state, query_start, query
FROM pg_stat_activity
WHERE state = 'idle'
ORDER BY query_start;
```

**Common Causes:**
- Connection leak in application
- Too many concurrent users
- Long-running queries holding connections
- Connection pool misconfiguration

### Scenario C: Slow Queries

```sql
-- Find slow queries
SELECT 
    pid,
    usename,
    state,
    query_start,
    now() - query_start AS duration,
    query
FROM pg_stat_activity
WHERE state = 'active'
  AND query_start < now() - interval '30 seconds'
ORDER BY query_start;
```

**Common Causes:**
- Missing indexes
- Table bloat
- Lock contention
- Vacuum/ANALYZE needed

### Scenario D: Lock Contention

```sql
-- Check for locks
SELECT 
    l.locktype,
    l.relation::regclass,
    l.mode,
    l.granted,
    a.usename,
    a.query,
    a.pid
FROM pg_locks l
JOIN pg_stat_activity a ON l.pid = a.pid
WHERE NOT l.granted;
```

---

## Resolution Steps

### Immediate Actions

1. **Restart Database** (if down)
   ```bash
   systemctl restart postgresql
   
   # Check status
   systemctl status postgresql
   ```

2. **Kill Blocking Queries** (if safe)
   ```sql
   -- Get blocking PIDs
   SELECT pg_terminate_backend(pid)
   FROM pg_stat_activity
   WHERE state = 'active'
     AND query_start < now() - interval '10 minutes';
   ```

3. **Release Idle Connections**
   ```sql
   -- Terminate idle connections (be careful!)
   SELECT pg_terminate_backend(pid)
   FROM pg_stat_activity
   WHERE state = 'idle'
     AND state_change < now() - interval '1 hour';
   ```

4. **Restart Application** (to clear connection pool)
   ```bash
   systemctl restart catalogizer-api
   ```

### Long-Term Fixes

1. **Tune Connection Pool**
   ```yaml
   # In config.json
   database:
     max_open_conns: 25
     max_idle_conns: 10
     conn_max_lifetime: "1h"
   ```

2. **Configure PostgreSQL**
   ```conf
   # postgresql.conf
   max_connections = 200
   shared_buffers = 256MB
   effective_cache_size = 1GB
   ```

3. **Add Connection Pooling (PgBouncer)**
   ```bash
   # Install PgBouncer
   apt-get install pgbouncer
   
   # Configure
   vi /etc/pgbouncer/pgbouncer.ini
   ```

### Performance Fixes

1. **Analyze Tables**
   ```sql
   ANALYZE media_items;
   ANALYZE files;
   ANALYZE users;
   ```

2. **Check for Missing Indexes**
   ```sql
   -- Check table statistics
   SELECT 
       schemaname,
       tablename,
       attname,
       n_distinct,
       correlation
   FROM pg_stats
   WHERE tablename = 'media_items';
   ```

3. **Vacuum Tables**
   ```sql
   VACUUM ANALYZE media_items;
   ```

---

## Verification

After resolution:

```bash
# Test connectivity
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT count(*) FROM media_items;"

# Check connection count
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT count(*) FROM pg_stat_activity;"

# Monitor for 5 minutes
watch -n 10 'psql -c "SELECT state, count(*) FROM pg_stat_activity GROUP BY state;"'
```

---

## Prevention

- Configure proper connection pool limits
- Set up PgBouncer for connection pooling
- Monitor slow queries and add indexes
- Regular VACUUM and ANALYZE
- Set up read replicas for heavy queries
- Implement query timeouts

---

## Escalation

If database remains inaccessible after 5 minutes:
1. Escalate to Database Administrator
2. Consider failover to standby (if configured)
3. Activate incident response
4. Check for data corruption

---

**Related Runbooks:**
- [API_UNRESPONSIVE.md](API_UNRESPONSIVE.md)
- SLOW_QUERIES (no runbook written yet)
- [HIGH_CPU.md](HIGH_CPU.md)
