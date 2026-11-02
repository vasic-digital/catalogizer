#!/bin/bash
# Dev/test launcher. Secrets are operator-supplied via the environment (read by
# NAME); there is no default admin password or JWT secret (CAT-021).
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD (not a published default)}"
: "${JWT_SECRET:?set JWT_SECRET (>= 32 chars)}"
export ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
export ADMIN_PASSWORD JWT_SECRET
export ENABLE_AUTH=true
export DATABASE_TYPE=sqlite
export DB_PATH=./data/catalogizer.db
exec ./test.bin -test-mode
