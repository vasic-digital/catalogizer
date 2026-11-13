# Security documentation index

Security guides, incident write-ups, and scan artefacts (gosec, govulncheck, nancy, npm audit, snyk).

Back to the documentation index: [docs/README.md](../README.md). Descriptions are copied mechanically from each file (its first heading and first prose line) unless overridden in scripts/docs/index_description_overrides.tsv; non-Markdown artefacts are listed by name and type without being read. 69 entries.

| File | Description |
|---|---|
| [2026-04-03-security-scan-consolidated.md](2026-04-03-security-scan-consolidated.md) | Security Scan Consolidated Report |
| [COMPREHENSIVE_SECURITY_AUDIT.md](COMPREHENSIVE_SECURITY_AUDIT.md) | Comprehensive Security Audit Report - A comprehensive security audit has been performed using 5 different security tools across all project components. The audit identified 4 standard library vulnerabilities... |
| [CORS_CONFIGURATION.md](CORS_CONFIGURATION.md) | CORS Configuration - Cross-Origin Resource Sharing is handled by the CORS() middleware defined in two locations: |
| [INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md) | Security Incident Response Plan - This document defines the security incident response procedures for Catalogizer deployments. It covers detection, containment, eradication, recovery, and post-incident... |
| [LEGACY_PASSWORD_HASH_SUNSET.md](LEGACY_PASSWORD_HASH_SUNSET.md) | Legacy SHA-256 password hash: sunset plan - The old POST /users stored hex(SHA-256(password + salt)) (64 lowercase hex |
| [MEMORY_LEAK_ANALYSIS.md](MEMORY_LEAK_ANALYSIS.md) | Memory Leak Analysis Report - Comprehensive memory leak detection performed using: |
| [PATH_CONFINEMENT.md](PATH_CONFINEMENT.md) | Filesystem path confinement (catalog-api) - Request-fed filesystem paths (sync endpoints, conversion jobs, cover-art lookup, asset cache, |
| [PENTEST_REPORT.md](PENTEST_REPORT.md) | Security Audit Baseline — Master Plan Phase 12 - Purpose. Master Plan v2 Phase 12 "Security Hardening & Penetration |
| [SECRETS_MANAGEMENT.md](SECRETS_MANAGEMENT.md) | Secrets Management - Catalogizer follows a strict precedence chain for all configuration values: |
| [SECURITY_HEADERS.md](SECURITY_HEADERS.md) | Security Headers - catalog-api sets security headers on every HTTP response via the SecurityHeaders() middleware in catalog-api/middleware/security_headers.go. |
| [SECURITY_REMEDIATION_SUMMARY.md](SECURITY_REMEDIATION_SUMMARY.md) | Security Remediation Summary - Successfully addressed all 7 HIGH severity security findings from the initial security scan (Scan ID: 20260210_172319). |
| [SECURITY_SCAN_REPORT.md](SECURITY_SCAN_REPORT.md) | Security Scan Report - Comprehensive security scanning has been performed across all project components. The results show LOW RISK with no critical or high-severity vulnerabilities detected.... |
| [SECURITY_SCAN_REPORT_2026-03-04.md](SECURITY_SCAN_REPORT_2026-03-04.md) | Security Scan Report — 2026-03-04 - All CRITICAL and HIGH severity findings have been addressed. The remaining MEDIUM and LOW findings are false positives or acceptable patterns (file paths from trusted... |
| [SECURITY_SCAN_REPORT_2026-03-26.md](SECURITY_SCAN_REPORT_2026-03-26.md) | Security Scan Report — 2026-03-26 - Consolidated security scan report covering all Catalogizer components. Two tools were executed directly (govulncheck, npm audit); container-based scanners (Snyk, Trivy,... |
| [firebase-api-key-exposure-20260629.html](firebase-api-key-exposure-20260629.html) | (HTML export; not read, listed by name only) |
| [firebase-api-key-exposure-20260629.md](firebase-api-key-exposure-20260629.md) | Security Incident — Firebase Android API Key in Git History - The live Firebase Android API key for com.catalogizer.androidtv |
| [firebase-api-key-exposure-20260629.pdf](firebase-api-key-exposure-20260629.pdf) | (PDF export; not read, listed by name only) |
| [gosec-20260210_172319.json](gosec-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [gosec-20260422_123551.json](gosec-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [gosec-20260429_080241.json](gosec-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [gosec-20260629_030611.json](gosec-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [gosec-baseline-2026-04-22.json](gosec-baseline-2026-04-22.json) | (JSON data artefact; not read, listed by name only) |
| [gosec-sample.md](gosec-sample.md) | gosec HIGH findings — baseline 2026-04-22 - Total: 24 \| Files scanned: 354 |
| [govulncheck-20260422_123551.txt](govulncheck-20260422_123551.txt) | (plain-text artefact; not read, listed by name only) |
| [govulncheck-20260429_080241.txt](govulncheck-20260429_080241.txt) | (plain-text artefact; not read, listed by name only) |
| [govulncheck-20260629_030611.txt](govulncheck-20260629_030611.txt) | (plain-text artefact; not read, listed by name only) |
| [nancy-20260422_123551.json](nancy-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [nancy-20260429_080241.json](nancy-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [nancy-20260629_030611.json](nancy-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalog-web-20260210_172319.json](npm-audit-catalog-web-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalog-web-20260422_123551.json](npm-audit-catalog-web-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalog-web-20260429_080241.json](npm-audit-catalog-web-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalog-web-20260629_030611.json](npm-audit-catalog-web-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalogizer-api-client-20260210_172319.json](npm-audit-catalogizer-api-client-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalogizer-api-client-20260422_123551.json](npm-audit-catalogizer-api-client-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalogizer-api-client-20260429_080241.json](npm-audit-catalogizer-api-client-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalogizer-api-client-20260629_030611.json](npm-audit-catalogizer-api-client-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalogizer-desktop-20260210_172319.json](npm-audit-catalogizer-desktop-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalogizer-desktop-20260422_123551.json](npm-audit-catalogizer-desktop-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalogizer-desktop-20260429_080241.json](npm-audit-catalogizer-desktop-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-catalogizer-desktop-20260629_030611.json](npm-audit-catalogizer-desktop-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-installer-wizard-20260210_172319.json](npm-audit-installer-wizard-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-installer-wizard-20260422_123551.json](npm-audit-installer-wizard-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-installer-wizard-20260429_080241.json](npm-audit-installer-wizard-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [npm-audit-installer-wizard-20260629_030611.json](npm-audit-installer-wizard-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [security-scan-20260210_172319.md](security-scan-20260210_172319.md) | Catalogizer Security Scan Report - This report contains the results of automated security scanning across all Catalogizer components. |
| [security-scan-20260422_123551.md](security-scan-20260422_123551.md) | Catalogizer Security Scan Report - This report contains the results of automated security scanning across all Catalogizer components. |
| [security-scan-20260429_080241.md](security-scan-20260429_080241.md) | Catalogizer Security Scan Report - This report contains the results of automated security scanning across all Catalogizer components. |
| [security-scan-20260629_030611.md](security-scan-20260629_030611.md) | Catalogizer Security Scan Report - This report contains the results of automated security scanning across all Catalogizer components. |
| [snyk-catalog-web-20260210_172319.json](snyk-catalog-web-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalog-web-20260422_123551.json](snyk-catalog-web-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalog-web-20260429_080241.json](snyk-catalog-web-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalog-web-20260629_030611.json](snyk-catalog-web-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalogizer-api-client-20260210_172319.json](snyk-catalogizer-api-client-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalogizer-api-client-20260422_123551.json](snyk-catalogizer-api-client-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalogizer-api-client-20260429_080241.json](snyk-catalogizer-api-client-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalogizer-api-client-20260629_030611.json](snyk-catalogizer-api-client-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalogizer-desktop-20260210_172319.json](snyk-catalogizer-desktop-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalogizer-desktop-20260422_123551.json](snyk-catalogizer-desktop-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalogizer-desktop-20260429_080241.json](snyk-catalogizer-desktop-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-catalogizer-desktop-20260629_030611.json](snyk-catalogizer-desktop-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-go-20260210_172319.json](snyk-go-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-go-20260422_123551.json](snyk-go-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-go-20260429_080241.json](snyk-go-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-go-20260629_030611.json](snyk-go-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-installer-wizard-20260210_172319.json](snyk-installer-wizard-20260210_172319.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-installer-wizard-20260422_123551.json](snyk-installer-wizard-20260422_123551.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-installer-wizard-20260429_080241.json](snyk-installer-wizard-20260429_080241.json) | (JSON data artefact; not read, listed by name only) |
| [snyk-installer-wizard-20260629_030611.json](snyk-installer-wizard-20260629_030611.json) | (JSON data artefact; not read, listed by name only) |
