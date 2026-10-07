#!/usr/bin/env bash
# roundtrip_postgres.sh - T132 (inside IMG-INFRA-CLIENT): write a row, read it back, update, delete, with SPECIFIED answers; a duplicate key and a wrong
# password are refused.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of POSTGRES postgres)"; N="$(nonce)"
q() { PGPASSWORD="$TI_POSTGRES_PASSWORD" PGCONNECT_TIMEOUT=8 psql -h "$H" -p 5432 -U "$TI_POSTGRES_USER" -d "$TI_POSTGRES_DB" -v ON_ERROR_STOP=1 -tA "$@"; }
val_is() { expect_eq "$(q -c "SELECT v FROM ti_rt WHERE id = 1")" "$1"; }
count_is() { expect_eq "$(q -c 'SELECT count(*) FROM ti_rt')" "$1"; }
# a refusal counts only with the SPECIFIC signal of the server (WF12 F8): an unreachable host or a crashed server also "fails", and must not pass as a refusal
refused_dup() { local o; o="$(q -c "INSERT INTO ti_rt VALUES (1, 'dup')" 2>&1)" && { echo "a duplicate key was accepted"; return 1; }
  case "$o" in *"duplicate key value violates unique constraint"*) ;; *) echo "refused, but not as a duplicate key: ${o:0:100}"; return 1;; esac; }
refused_pw() { local o; o="$(PGPASSWORD=wrong-credential-1 PGCONNECT_TIMEOUT=8 psql -h "$H" -p 5432 -U "$TI_POSTGRES_USER" -d "$TI_POSTGRES_DB" -tAc 'SELECT 1' 2>&1)" && { echo "a wrong password was accepted"; return 1; }
  case "$o" in *"password authentication failed"*) ;; *) echo "refused, but not by authentication: ${o:0:100}"; return 1;; esac; }
step create_table q -c 'CREATE TABLE ti_rt (id integer PRIMARY KEY, v text NOT NULL)'
step insert_row q -c "INSERT INTO ti_rt VALUES (1, '$N')"
step read_back val_is "$N"
step update_row q -c "UPDATE ti_rt SET v = 'updated-$N' WHERE id = 1"
step read_updated val_is "updated-$N"
step duplicate_key_refused refused_dup
step delete_row q -c 'DELETE FROM ti_rt WHERE id = 1'
step count_zero count_is 0
step drop_table q -c 'DROP TABLE ti_rt'
step wrong_password_refused refused_pw
finish postgres
