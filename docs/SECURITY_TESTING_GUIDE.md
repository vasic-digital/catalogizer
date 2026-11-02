# Security Testing Guide for Catalogizer

This guide provides comprehensive information about security testing integrated into the Catalogizer project.

## Overview

Catalogizer includes comprehensive security testing using industry-standard tools:
- **SonarQube**: Code quality and security analysis
- **Snyk**: Vulnerability scanning for dependencies and code
- **Trivy**: Container and filesystem security scanning
- **OWASP Dependency Check**: Third-party dependency vulnerability analysis

## Prerequisites

### Required Tools
- Docker & Docker Compose
- Node.js 18+
- Go 1.21+
- Java 21+ (for Android tests)

### Environment Variables
Set these environment variables for full functionality:

```bash
# SonarQube (optional for freemium)
export SONAR_TOKEN="your-sonarqube-token"
export SONAR_HOST_URL="http://localhost:9000"

# Snyk (optional for freemium)
export SNYK_TOKEN="your-snyk-token"
export SNYK_ORG="catalogizer"
export SNYK_SEVERITY_THRESHOLD="medium"

# Database and application secrets
export JWT_SECRET="your-jwt-secret-here"
export ADMIN_PASSWORD="<your-admin-password>"
export SMB_PASSWORD="your-smb-password-here"
export FTP_PASSWORD="your-ftp-password-here"
export WEBDAV_PASSWORD="your-webdav-password-here"
export SYNOLOGY_SECRET="your-synology-secret-here"
```

## Running Security Tests

### Quick Start
Run the complete test suite including security scans:

```bash
./scripts/run-all-tests.sh
```

### Individual Security Scans

#### SonarQube Analysis
```bash
./scripts/sonarqube-scan.sh
```

#### Snyk Security Scan
```bash
./scripts/snyk-scan.sh
```

#### Comprehensive Security Testing
```bash
./scripts/security-test.sh
```

## Security Services

### Starting Security Services
```bash
docker compose -f docker-compose.security.yml up -d
```

### Stopping Security Services
```bash
docker compose -f docker-compose.security.yml down
```

## Test Reports

All security reports are generated in the `reports/` directory:

- `comprehensive-test-report-*.html` - Complete test suite results
- `sonarqube-report.json` - SonarQube analysis results
- `snyk-*.json` - Snyk vulnerability scan results
- `snyk-security-report.html` - Snyk HTML report
- `trivy-results.json` - Trivy scan results
- `dependency-check/` - OWASP Dependency Check reports

## Security Best Practices

### Code Security
1. **Never commit secrets** - Use environment variables
2. **Validate all inputs** - Prevent injection attacks
3. **Use parameterized queries** - Prevent SQL injection
4. **Implement proper authentication** - JWT-based auth with strong secrets
5. **Enable HTTPS** - Encrypt all communications

### Dependency Management
1. **Regular updates** - Keep dependencies up to date
2. **Vulnerability scanning** - Automated scanning in CI/CD
3. **License compliance** - Check dependency licenses
4. **Minimal dependencies** - Reduce attack surface

### Container Security
1. **Minimal base images** - Reduce vulnerability surface
2. **Non-root users** - Run containers with least privilege
3. **Security scanning** - Scan images before deployment
4. **Resource limits** - Prevent resource exhaustion attacks

## Freemium Limitations

The security testing is configured for freemium usage:

### SonarQube Community Edition
- Unlimited code analysis
- Basic security hotspot detection
- Limited quality gate rules

### Snyk Free Tier
- Up to 100 projects
- Limited scans per month
- Community support
- Basic vulnerability detection

## Troubleshooting

### Common Issues

#### Docker Memory Issues
Increase Docker memory allocation:
```bash
# For OWASP Dependency Check
docker compose -f docker-compose.security.yml up -d dependency-check
```

#### Authentication Failures
Check environment variables:
```bash
echo $SONAR_TOKEN
echo $SNYK_TOKEN
```

#### Service Startup Issues
Check service logs:
```bash
docker compose -f docker-compose.security.yml logs sonarqube
docker compose -f docker-compose.security.yml logs snyk-cli
```

### Performance Optimization

#### Faster Scans
- Exclude test files and dependencies
- Use `.snyk` and `.sonarqube` configuration files
- Limit scan scope with proper exclusions

#### Resource Management
- Adjust memory limits in `docker-compose.security.yml`
- Use parallel scanning for large codebases
- Cache scan results where possible

## Integration with CI/CD

### Local Security Scanning

> **Note:** GitHub Actions are permanently disabled for this project. Run security tests locally using the commands below.

```bash
# Run all tests including security scans
export SONAR_TOKEN=your_sonar_token
export SNYK_TOKEN=your_snyk_token
./scripts/run-all-tests.sh
```

### Jenkins Pipeline Example
```groovy
pipeline {
    agent any
    environment {
        SONAR_TOKEN = credentials('sonar-token')
        SNYK_TOKEN = credentials('snyk-token')
    }
    stages {
        stage('Security Tests') {
            steps {
                sh './scripts/run-all-tests.sh'
            }
        }
    }
}
```

## Security Monitoring

### Continuous Monitoring
1. **Daily scans** - Automated vulnerability scanning
2. **Dependency updates** - Automated security updates
3. **Security alerts** - Email notifications for new vulnerabilities
4. **Compliance reporting** - Regular security reports

### Incident Response
1. **Vulnerability assessment** - Quick impact analysis
2. **Remediation planning** - Prioritized fix schedule
3. **Security patches** - Emergency patch deployment
4. **Post-incident review** - Lessons learned documentation

## Support and Resources

### Documentation
- [SonarQube Documentation](https://docs.sonarqube.org/)
- [Snyk Documentation](https://support.snyk.io/)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [OWASP Dependency Check](https://jeremylong.github.io/DependencyCheck/)

### Community
- [SonarQube Community](https://community.sonarsource.com/)
- [Snyk Community](https://community.snyk.io/)
- [OWASP Foundation](https://owasp.org/)

### Security Contacts
- Security Team: security@catalogizer.com
- Bug Reports: https://github.com/catalogizer/security/issues
- Security Questions: security@catalogizer.com

---

**Note**: This guide is regularly updated. Check for the latest version in the project repository.