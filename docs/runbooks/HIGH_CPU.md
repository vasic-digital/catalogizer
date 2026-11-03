# Runbook: High CPU Usage

**Alert:** HighCPUUsage / CriticalCPUUsage  
**Severity:** Warning (80%) / Critical (95%)  
**Service:** System / Application  

---

## Summary

This alert fires when CPU usage exceeds defined thresholds (80% for warning, 95% for critical) for a sustained period.

---

## Initial Assessment (2 minutes)

### 1. Verify the Alert
```bash
# Check current CPU usage
ssh <affected-host>
top -bn1 | head -20

# Or use Prometheus query
# 100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```

### 2. Identify the Process
```bash
# Find top CPU-consuming processes
ps aux --sort=-%cpu | head -10

# Check if it's a Catalogizer process
pgrep -f catalogizer-api
pgrep -f catalogizer-web
```

### 3. Check Recent Changes
- Recent deployments?
- Configuration changes?
- Increased traffic/load?

---

## Diagnosis Steps

### Scenario A: Catalogizer API Process

```bash
# Check API specific metrics
curl http://localhost:8080/metrics | grep -E "(cpu|goroutine|gc)"

# Check active connections
ss -tuln | grep 8080
netstat -an | grep :8080 | wc -l

# Check request rate
# Look for: rate(http_requests_total[5m])
```

**Possible Causes:**
- High request volume (DDoS, legitimate spike)
- Expensive query running
- Infinite loop in code
- Memory pressure causing GC overhead

### Scenario B: Database Process

```bash
# Check active queries
# For PostgreSQL:
sudo -u postgres psql -c "SELECT pid, state, query_start, query FROM pg_stat_activity WHERE state = 'active';"

# Check slow queries
# Look in PostgreSQL slow query log
```

**Possible Causes:**
- Missing index causing table scans
- Long-running transaction
- Lock contention
- Vacuum/maintenance running

### Scenario C: System Process

```bash
# Check system logs
journalctl -xe --since "1 hour ago"
dmesg | tail -50
```

**Possible Causes:**
- System maintenance (backups, updates)
- Malware/mining
- Hardware issues

---

## Resolution Steps

### Immediate Actions

1. **Scale horizontally** (if applicable)
   ```bash
   # Add more API instances
   podman-compose up -d --scale catalogizer-api=3
   ```

2. **Rate limiting** (if traffic spike)
   ```bash
   # Enable stricter rate limiting
   curl -X POST http://localhost:8080/admin/rate-limit/enable \
     -H "Authorization: Bearer $ADMIN_TOKEN"
   ```

3. **Restart service** (if memory leak suspected)
   ```bash
   # Graceful restart
   systemctl restart catalogizer-api
   # OR
   podman-compose restart catalogizer-api
   ```

### Database-Related

1. **Kill long-running queries** (if safe)
   ```sql
   -- PostgreSQL
   SELECT pg_terminate_backend(pid) FROM pg_stat_activity 
   WHERE state = 'active' AND query_start < now() - interval '5 minutes';
   ```

2. **Analyze tables**
   ```sql
   ANALYZE media_items;
   ANALYZE files;
   ```

### Code-Related

If a specific endpoint is causing issues:

1. **Check logs**
   ```bash
   grep "ERROR" /var/log/catalogizer/api.log | tail -50
   ```

2. **Profile the application**
   ```bash
   # Enable pprof
   curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
   ```

---

## Verification

After taking action, verify:

```bash
# Monitor CPU for 5 minutes
watch -n 5 'top -bn1 | grep "Cpu(s)"'

# Check if alert resolves in Prometheus
# Query: 100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```

---

## Prevention

- Set up horizontal auto-scaling
- Implement circuit breakers for expensive operations
- Add caching for frequently accessed data
- Regular performance profiling
- Capacity planning reviews

---

## Escalation

If issue persists after 15 minutes:
1. Escalate to Engineering Manager
2. Consider fail-over to standby systems
3. Notify users if service degradation occurs

---

**Related Runbooks:**
- [MEMORY_LEAK.md](MEMORY_LEAK.md)
- [API_UNRESPONSIVE.md](API_UNRESPONSIVE.md)
- SLOW_QUERIES (no runbook written yet)
