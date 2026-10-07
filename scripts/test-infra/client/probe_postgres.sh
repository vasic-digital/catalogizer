#!/usr/bin/env bash
# probe_postgres.sh - T128 (runs inside IMG-INFRA-CLIENT): a real `SELECT 1` over the PostgreSQL wire protocol. An open TCP port is not a pass.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of POSTGRES postgres)"
out="$(PGPASSWORD="$TI_POSTGRES_PASSWORD" PGCONNECT_TIMEOUT=8 psql -h "$H" -p 5432 -U "$TI_POSTGRES_USER" -d "$TI_POSTGRES_DB" -tAc 'SELECT 1' 2>&1)"; rc=$?
[ "$rc" = 0 ] && [ "$out" = 1 ] || fail "postgres SELECT 1 rc=$rc answer='${out:0:200}'"
pass "postgres SELECT 1 answered 1"
