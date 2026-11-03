# Runbook: Security Breach / Incident Response

**Alert:** SQLInjectionAttempt / XSSAttempt / PathTraversalAttempt / PrivilegeEscalationAttempt / PotentialBruteForce  
**Severity:** Critical  
**Category:** Security  

---

## ⚠️ CRITICAL ALERT ⚠️

This is a **CRITICAL SECURITY ALERT**. Immediate action required.

---

## Immediate Actions (First 5 Minutes)

### 1. Acknowledge and Assess

```bash
# Check the alert details
echo "Alert: $ALERT_NAME"
echo "Source: $SOURCE_IP"
echo "Time: $TIMESTAMP"

# Check if this is ongoing
tail -f /var/log/catalogizer/security.log | grep "$SOURCE_IP"
```

### 2. Preserve Evidence

```bash
# Create incident directory
INCIDENT_DIR="/var/incidents/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$INCIDENT_DIR"

# Collect logs
cp /var/log/catalogizer/api.log "$INCIDENT_DIR/"
cp /var/log/catalogizer/security.log "$INCIDENT_DIR/"
cp /var/log/nginx/access.log "$INCIDENT_DIR/" 2>/dev/null

# Collect network data
ss -tan > "$INCIDENT_DIR/network-connections.txt"
netstat -tan > "$INCIDENT_DIR/netstat.txt"

# Save current state
ps aux > "$INCIDENT_DIR/processes.txt"
df -h > "$INCIDENT_DIR/disk-usage.txt"
```

### 3. Notify Security Team

- **Slack:** @channel in #security-alerts
- **Email:** security@catalogizer.io
- **Phone:** Call security on-call

---

## Incident Classification

### Type A: Active Attack (Ongoing)
- Multiple attempts from same source
- Successful exploitation detected
- Data exfiltration suspected

**Action:** Immediate containment required

### Type B: Failed Attack Attempt
- Single or few attempts
- All attacks blocked
- No successful exploitation

**Action:** Monitor and document

### Type C: Potential Insider Threat
- Privilege escalation attempt
- Unusual admin activity
- Data access anomalies

**Action:** Immediate investigation and potential account suspension

---

## Containment Procedures

### For External Attacks

1. **Block Source IP**
   ```bash
   # Using iptables
   iptables -A INPUT -s $SOURCE_IP -j DROP
   
   # Using firewall-cmd
   firewall-cmd --add-rich-rule="rule family='ipv4' source address='$SOURCE_IP' reject"
   
   # Using fail2ban (if configured)
   fail2ban-client set catalogizer banip $SOURCE_IP
   ```

2. **Enable Rate Limiting** (if not already)
   ```bash
   # Enable strict rate limiting
   curl -X POST http://localhost:8080/admin/rate-limit/strict \
     -H "Authorization: Bearer $ADMIN_TOKEN"
   ```

3. **Enable WAF Rules** (if available)
   ```bash
   # Enable additional WAF protections
   # (implementation depends on WAF solution)
   ```

### For Successful Exploitation

1. **Isolate Affected Systems**
   ```bash
   # Take affected instance out of load balancer
   # (implementation depends on infrastructure)
   ```

2. **Preserve Evidence**
   ```bash
   # Create disk snapshots
   # (implementation depends on cloud provider)
   ```

3. **Disable Compromised Accounts**
   ```bash
   # Disable user account
   curl -X POST http://localhost:8080/admin/users/$USER_ID/disable \
     -H "Authorization: Bearer $ADMIN_TOKEN"
   
   # Invalidate all sessions for user
   curl -X POST http://localhost:8080/admin/users/$USER_ID/sessions/revoke \
     -H "Authorization: Bearer $ADMIN_TOKEN"
   ```

---

## Investigation Steps

### 1. Log Analysis

```bash
# Search for attacker activity
grep "$SOURCE_IP" /var/log/catalogizer/*.log

# Check authentication attempts
grep "$SOURCE_IP" /var/log/catalogizer/auth.log

# Check for successful logins
grep -E "(login|auth).*success.*$SOURCE_IP" /var/log/catalogizer/api.log
```

### 2. Database Investigation

```sql
-- Check for unauthorized data access
SELECT 
    user_id,
    action,
    resource,
    timestamp,
    ip_address
FROM audit_log
WHERE ip_address = '$SOURCE_IP'
  AND timestamp > now() - interval '1 hour'
ORDER BY timestamp DESC;

-- Check for data modifications
SELECT 
    table_name,
    operation,
    user_id,
    timestamp
FROM audit_log
WHERE operation IN ('UPDATE', 'DELETE', 'INSERT')
  AND timestamp > now() - interval '1 hour'
ORDER BY timestamp DESC;
```

### 3. File System Check

```bash
# Check for modified files
find /opt/catalogizer -type f -mtime -1 -ls

# Check for new files
find /opt/catalogizer -type f -newer /var/log/catalogizer/api.log -ls

# Check file integrity (if AIDE configured)
aide --check
```

### 4. Network Analysis

```bash
# Check active connections from source
ss -tan | grep "$SOURCE_IP"

# Check for data exfiltration
# (analyze network traffic if packet capture available)
```

---

## Specific Attack Responses

### SQL Injection

```bash
# Check for successful injection
grep -i "union\|select.*from\|drop.*table" /var/log/catalogizer/api.log

# Verify database integrity
psql -c "\dt"  # List tables
psql -c "SELECT count(*) FROM media_items;"  # Check row counts
```

### XSS (Cross-Site Scripting)

```bash
# Check for stored XSS payloads
grep -i "<script\|javascript:\|onerror\|onload" /var/log/catalogizer/api.log

# Check database for malicious content
psql -c "SELECT id, title FROM media_items WHERE title LIKE '%<%';"
```

### Path Traversal

```bash
# Check for unauthorized file access
grep "../\|..\\" /var/log/catalogizer/api.log

# Check for accessed sensitive files
grep -E "/etc/passwd|/etc/shadow|.env|config.json" /var/log/catalogizer/api.log
```

### Brute Force

```bash
# Check failed login attempts
grep "authentication.*failed.*$SOURCE_IP" /var/log/catalogizer/auth.log | wc -l

# Check if any were successful
grep "authentication.*success.*$SOURCE_IP" /var/log/catalogizer/auth.log
```

---

## Recovery Procedures

### After Containment

1. **Patch Vulnerabilities**
   - Deploy security patches
   - Update WAF rules
   - Review and fix vulnerable code

2. **Reset Credentials**
   - Force password reset for affected accounts
   - Rotate API keys
   - Update database passwords
   - Rotate JWT secrets

3. **Verify System Integrity**
   ```bash
   # Run security scan
   ./scripts/run-all-security-scans.sh
   
   # Check file integrity
   aide --check
   
   # Verify database integrity
   psql -c "SELECT pg_database.datname, pg_database_size(pg_database.datname) FROM pg_database WHERE datname = 'catalogizer';"
   ```

4. **Restore Service**
   - Remove IP blocks (if safe)
   - Restore normal rate limits
   - Monitor closely

---

## Documentation Requirements

### Incident Report Must Include:

1. **Timeline**
   - When attack started
   - When detected
   - Containment time
   - Resolution time

2. **Impact Assessment**
   - Data accessed/modified
   - Systems affected
   - User impact

3. **Technical Details**
   - Attack vector
   - Exploited vulnerabilities
   - Evidence preserved

4. **Response Actions**
   - Containment steps
   - Investigation findings
   - Recovery actions

5. **Recommendations**
   - Security improvements
   - Process changes
   - Training needs

---

## Post-Incident Actions

1. **Notify Stakeholders**
   - If user data affected, notify users
   - Regulatory notifications (if required)
   - Update security page

2. **Conduct Post-Mortem**
   - Schedule within 48 hours
   - Include all responders
   - Document lessons learned

3. **Implement Improvements**
   - Security patches
   - Monitoring improvements
   - Process updates

4. **Update Runbook**
   - Document new attack patterns
   - Improve response procedures

---

## Prevention

- Regular security scanning
- Web Application Firewall (WAF)
- Input validation and sanitization
- Principle of least privilege
- Regular security training
- Penetration testing
- Bug bounty program

---

## Emergency Contacts

| Role | Contact | Phone |
|------|---------|-------|
| Security Lead | security@catalogizer.io | +1-XXX-XXX-XXXX |
| Engineering Manager | manager@catalogizer.io | +1-XXX-XXX-XXXX |
| Legal | legal@catalogizer.io | +1-XXX-XXX-XXXX |
| PR/Communications | pr@catalogizer.io | +1-XXX-XXX-XXXX |

---

**Related Runbooks:**
- BRUTE_FORCE (no runbook written yet)
- DOS_ATTACK (no runbook written yet)

**External Resources:**
- NIST Incident Response Guide
- OWASP Incident Response
- SANS Incident Handler's Handbook
