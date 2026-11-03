# Alert Runbooks Index

This directory contains runbooks for responding to alerts and incidents in the Catalogizer system.

## Quick Reference

| Alert | Severity | Runbook | Response Time |
|-------|----------|---------|---------------|
| CriticalCPUUsage | 🔴 Critical | [HIGH_CPU.md](HIGH_CPU.md) | Immediate |
| CriticalMemoryUsage | 🔴 Critical | [MEMORY_LEAK.md](MEMORY_LEAK.md) | Immediate |
| APIDown | 🔴 Critical | [API_UNRESPONSIVE.md](API_UNRESPONSIVE.md) | Immediate |
| DatabaseConnectionFailure | 🔴 Critical | [DATABASE_CONNECTION.md](DATABASE_CONNECTION.md) | Immediate |
| SQLInjectionAttempt | 🔴 Critical | [SECURITY_BREACH.md](SECURITY_BREACH.md) | Immediate |
| HighCPUUsage | 🟠 High | [HIGH_CPU.md](HIGH_CPU.md) | 15 min |
| HighMemoryUsage | 🟠 High | [MEMORY_LEAK.md](MEMORY_LEAK.md) | 15 min |
| HighErrorRate | 🟠 High | [API_UNRESPONSIVE.md](API_UNRESPONSIVE.md) | 15 min |

## Runbook Categories

### System Performance
- [HIGH_CPU.md](HIGH_CPU.md) - High CPU utilization
- [MEMORY_LEAK.md](MEMORY_LEAK.md) - Memory leaks and high memory usage
- [DISK_SPACE.md](DISK_SPACE.md) - Disk space issues

### Application Issues
- [API_UNRESPONSIVE.md](API_UNRESPONSIVE.md) - API downtime and performance
- [DATABASE_CONNECTION.md](DATABASE_CONNECTION.md) - Database connectivity issues
- SLOW_QUERIES - Database performance issues (no runbook written yet; see docs/FOLLOW_UPS_NEXT_PHASE.md "Documentation follow-ups")

### Security Incidents
- [SECURITY_BREACH.md](SECURITY_BREACH.md) - Security breach response
- BRUTE_FORCE - (no runbook written yet) Brute force attack response
- DOS_ATTACK - (no runbook written yet) Denial of service response

### Infrastructure
- NODE_DOWN - (no runbook written yet) Node/Host failures
- NETWORK_ISSUES - (no runbook written yet) Network connectivity
- CERTIFICATE_EXPIRY - (no runbook written yet) SSL certificate issues

## Escalation Matrix

| Severity | First Response | Escalation | Time to Escalate |
|----------|---------------|------------|------------------|
| Critical | On-call Engineer | Engineering Manager | 15 minutes |
| High | On-call Engineer | Tech Lead | 30 minutes |
| Warning | Automated/System | On-call Engineer | 2 hours |

## Contact Information

- **On-call Engineer**: See PagerDuty rotation
- **Engineering Manager**: manager@catalogizer.io
- **Security Team**: security@catalogizer.io
- **Infrastructure Team**: infrastructure@catalogizer.io

## Communication Channels

- **Critical Alerts**: #critical-alerts (Slack) + Phone call
- **High Alerts**: #high-priority-alerts (Slack)
- **Security Alerts**: #security-alerts (Slack)
- **General Alerts**: #alerts (Slack)

## Post-Incident Process

1. Acknowledge the alert
2. Follow the runbook
3. Resolve the issue
4. Document findings
5. Schedule post-mortem (for critical incidents)
6. Update runbook if needed

---

*Last Updated: April 10, 2026*
