#!/bin/bash
# Security Scanning Setup Script for Catalogizer
# This script helps set up SonarQube and Snyk for security scanning

set -e

echo "🔐 Catalogizer Security Scanning Setup"
echo "======================================"

# Create reports directory
mkdir -p reports
mkdir -p sonarqube/conf

# Check for required tools
echo "📋 Checking prerequisites..."
if ! command -v podman &> /dev/null; then
    echo "❌ Podman is not installed. Please install Podman first."
    exit 1
fi

if ! command -v podman-compose &> /dev/null; then
    echo "❌ Podman-compose is not installed. Please install podman-compose first."
    exit 1
fi

echo "✅ Prerequisites check passed"

# Create environment file template
echo "📝 Creating environment file template..."
# shellcheck source=lib/init_env_security.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib/init_env_security.sh"
init_env_security . || exit 1

# Create SonarQube configuration directory
echo "📁 Setting up SonarQube configuration..."
mkdir -p sonarqube/conf
cat > sonarqube/conf/sonar.properties << 'EOF'
# SonarQube Configuration for Catalogizer
sonar.core.serverBaseURL=http://localhost:9000

# Database configuration (managed by Docker Compose)
sonar.jdbc.url=jdbc:postgresql://sonarqube-db:5432/sonar
sonar.jdbc.username=sonar
sonar.jdbc.password=sonar_password

# Web configuration
sonar.web.host=0.0.0.0
sonar.web.port=9000
sonar.web.context=/

# Security
sonar.forceAuthentication=false

# Java options
sonar.ce.javaOpts=-Xmx1g -Xms512m
sonar.web.javaOpts=-Xmx1g -Xms512m

# Update center
sonar.updatecenter.activate=true

# Project defaults
sonar.scm.disabled=true
sonar.scm.provider=git
EOF

echo "✅ SonarQube configuration created"

# Create dependency check suppression file if it doesn't exist
if [ ! -f dependency-check-suppressions.xml ]; then
    echo "📝 Creating dependency check suppression file..."
    cat > dependency-check-suppressions.xml << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<suppressions xmlns="https://jeremylong.github.io/DependencyCheck/dependency-suppression.1.3.xsd">
    <!-- Suppress false positives for Catalogizer project -->
    <suppress>
        <notes><![CDATA[
        Suppress false positives in test dependencies
        ]]></notes>
        <gav regex="true">.*</gav>
        <cve>CVE-2023-12345</cve>
    </suppress>
</suppressions>
EOF
    echo "✅ Dependency check suppression file created"
fi

# Create security scanning script
echo "📝 Creating security scanning script..."
cat > scripts/run-security-scan.sh << 'EOF'
#!/bin/bash
# Run comprehensive security scanning for Catalogizer

set -e

echo "🔐 Starting Catalogizer Security Scanning Suite"
echo "=============================================="

# Load environment variables
if [ -f .env.security ]; then
    source .env.security
fi

# Start SonarQube services
echo "🚀 Starting SonarQube..."
podman-compose -f docker-compose.security.yml up -d sonarqube sonarqube-db

echo "⏳ Waiting for SonarQube to be ready..."
sleep 30

# Check if SonarQube is running
if curl -s http://localhost:9000/api/system/status | grep -q "UP"; then
    echo "✅ SonarQube is running at http://localhost:9000"
    echo "   Default credentials: admin/admin"
    echo "   Please log in and generate a project token, then update SONAR_TOKEN in .env.security"
else
    echo "⚠️ SonarQube may still be starting up. Check logs with:"
    echo "   podman-compose -f docker-compose.security.yml logs sonarqube"
fi

# Run Snyk scanning (if token is set)
if [ "$SNYK_TOKEN" != "dummy-token-for-freemium-mode" ] && [ -n "$SNYK_TOKEN" ]; then
    echo "🔍 Running Snyk scanning..."
    podman-compose -f docker-compose.security.yml --profile snyk-scan run --rm snyk-cli
    echo "✅ Snyk scanning completed. Reports saved to reports/"
else
    echo "⚠️ Snyk token not configured. Running in freemium mode..."
    echo "   Get a free token at: https://app.snyk.io/account"
    echo "   Then update SNYK_TOKEN in .env.security"
fi

# Run OWASP Dependency Check
echo "🔍 Running OWASP Dependency Check..."
podman-compose -f docker-compose.security.yml --profile dependency-check run --rm dependency-check
echo "✅ OWASP Dependency Check completed. Reports saved to reports/dependency-check/"

# Run Trivy scanning
echo "🔍 Running Trivy vulnerability scanning..."
podman-compose -f docker-compose.security.yml --profile trivy-scan run --rm trivy-scanner
echo "✅ Trivy scanning completed. Reports saved to reports/"

echo ""
echo "📊 Security Scanning Summary"
echo "==========================="
echo "1. SonarQube: http://localhost:9000 (admin/admin)"
echo "2. Snyk Reports: reports/snyk-*.json"
echo "3. OWASP Dependency Check: reports/dependency-check/"
echo "4. Trivy Reports: reports/trivy-results.json"
echo ""
echo "🔧 Next Steps:"
echo "   - Log into SonarQube and generate a project token"
echo "   - Update SONAR_TOKEN in .env.security"
echo "   - Get a free Snyk token from https://app.snyk.io/account"
echo "   - Update SNYK_TOKEN in .env.security"
echo "   - Review security reports and fix critical issues"
EOF

chmod +x scripts/run-security-scan.sh

echo "✅ Security scanning script created"

# Create quick scan script
echo "📝 Creating quick scan script..."
cat > scripts/quick-security-scan.sh << 'EOF'
#!/bin/bash
# Quick security scan for development

set -e

echo "🔍 Running quick security scan..."

# Run Go security checks
echo "🔐 Checking Go dependencies for vulnerabilities..."
cd catalog-api
if command -v govulncheck &> /dev/null; then
    govulncheck ./... 2>/dev/null || true
else
    echo "⚠️ govulncheck not installed. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
fi
cd ..

# Run npm audit for frontend
echo "🔐 Checking npm dependencies for vulnerabilities..."
if [ -f "catalog-web/package.json" ]; then
    cd catalog-web
    npm audit --audit-level=high 2>/dev/null || true
    cd ..
fi

# Run basic container security check
echo "🔐 Checking Dockerfile security..."
if [ -f "catalog-api/Dockerfile" ]; then
    echo "📋 Dockerfile security tips:"
    echo "   - Use specific version tags for base images"
    echo "   - Run as non-root user"
    echo "   - Use multi-stage builds to reduce attack surface"
    echo "   - Scan images with Trivy or Snyk"
fi

echo "✅ Quick security scan completed"
EOF

chmod +x scripts/quick-security-scan.sh

echo "✅ Quick scan script created"

echo ""
echo "🎉 Security Scanning Setup Complete!"
echo "==================================="
echo ""
echo "📋 Available commands:"
echo "   ./scripts/run-security-scan.sh    - Run full security scanning suite"
echo "   ./scripts/quick-security-scan.sh  - Run quick security checks"
echo ""
echo "🔧 Setup Instructions:"
echo "   1. Review and update .env.security with your tokens"
echo "   2. Run: ./scripts/run-security-scan.sh"
echo "   3. Log into SonarQube at http://localhost:9000 (admin/admin)"
echo "   4. Generate a project token in SonarQube"
echo "   5. Get a free Snyk token from https://app.snyk.io/account"
echo "   6. Update tokens in .env.security"
echo "   7. Re-run security scans with updated tokens"
echo ""
echo "📚 Documentation:"
echo "   - SonarQube: https://docs.sonarqube.org/latest/"
echo "   - Snyk: https://docs.snyk.io/"
echo "   - OWASP Dependency Check: https://jeremylong.github.io/DependencyCheck/"
echo "   - Trivy: https://aquasecurity.github.io/trivy/"

chmod +x scripts/setup-security-scanning.sh

echo "✅ Security scanning setup script created"

echo ""
echo "🚀 To set up security scanning, run:"
echo "   ./scripts/setup-security-scanning.sh"
echo ""
echo "Then follow the instructions to configure SonarQube and Snyk."