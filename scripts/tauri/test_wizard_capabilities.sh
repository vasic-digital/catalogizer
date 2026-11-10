#!/usr/bin/env bash
# Guard (CAT-TAURI-INSTALLER-1): every Tauri plugin command the installer-wizard frontend calls must be granted by a
# capability file, otherwise Tauri 2 answers "Command plugin:<name>|<cmd> not allowed by ACL" at runtime (measured live
# 2026-11-06: Save Configuration failed with "Command plugin:dialog|save not allowed by ACL").
# Scope: plugin-dialog open/save, the only plugin the wizard frontend imports. Exit 0 PASS / 1 FAIL.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
W="$ROOT/installer-wizard"
CAPS="$W/src-tauri/capabilities"
fail=0
need=()
grep -rqE "import \{[^}]*\bsave\b[^}]*\} from '@tauri-apps/plugin-dialog'" "$W/src" --include=*.ts --include=*.tsx --exclude-dir=test && need+=(dialog:allow-save)
grep -rqE "import \{[^}]*\bopen\b[^}]*\} from '@tauri-apps/plugin-dialog'" "$W/src" --include=*.ts --include=*.tsx --exclude-dir=test && need+=(dialog:allow-open)
# control needle: the scan must see at least one dialog import in the wizard source, else it is blind
[ "${#need[@]}" -gt 0 ] || { echo "BLIND: no plugin-dialog import found under $W/src" >&2; exit 2; }
for p in "${need[@]}"; do
  if ! cat "$CAPS"/*.json 2>/dev/null | grep -q "\"$p\""; then echo "FAIL: permission $p is used by the frontend but granted by no capability in $CAPS" >&2; fail=1; fi
done
if [ "$fail" = 0 ]; then
  cat "$CAPS"/*.json | grep -q '"main"' || { echo "FAIL: no capability targets window \"main\"" >&2; fail=1; }
fi
[ "$fail" = 0 ] && echo "PASS: ${need[*]} granted"
exit "$fail"
