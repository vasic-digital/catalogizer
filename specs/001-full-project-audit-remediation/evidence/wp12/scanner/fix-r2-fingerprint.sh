#!/usr/bin/env bash
# fix-r2-fingerprint.sh [--stamp <logfile> <command text>]
# Prints the fingerprint of the tree the evidence was produced from: HEAD, then the sha256 of every modified or untracked file under catalog-api/,
# the settings/scanner docs and the filesystem submodule's working tree, and ONE combined hash. With --stamp it PREPENDS "command, fingerprint" to the
# log (WF22 T3: logs used to carry neither the command nor the tree they were produced from).
cd /home/milosvasic/Projects/catalogizer || exit 9
files() {
  { git status --porcelain --untracked-files=all -- catalog-api docs/scripts/generic_scanner.md docs/scripts/settings_contract.md Website/guides/configuration.md docs/ADMIN_GUIDE.md docs/guides/PROTOCOL_IMPLEMENTATION_GUIDE.md | awk '{print $2}'
    git -C submodules/filesystem status --porcelain --untracked-files=all | awk '{print "submodules/filesystem/" $2}'; } | sort -u
}
print_fp() {
  echo "# fingerprint $(date -u +%FT%TZ) HEAD=$(git rev-parse --short HEAD) files=$(files | wc -l) submodules/filesystem: HEAD=$(git -C submodules/filesystem rev-parse --short HEAD) pointer-in-HEAD=$(git ls-tree HEAD submodules/filesystem | awk '{print substr($3,1,7)}') dirty-files=$(git -C submodules/filesystem status --porcelain --untracked-files=all | wc -l)"
  files | while read -r f; do [ -f "$f" ] && sha256sum "$f"; done
}
fp=$(print_fp)
mine=$(printf '%s\n' "$fp" | grep -v '^#' | grep -v ' submodules/filesystem/' | sha256sum | cut -c1-16)
sub=$(printf '%s\n' "$fp" | grep -v '^#' | grep ' submodules/filesystem/' | sha256sum | cut -c1-16)
subhead=$(git -C submodules/filesystem rev-parse --short HEAD)
combined="catalog-api+docs=$mine submodules/filesystem@$subhead(+dirty working tree hash $sub)"
if [ "${1:-}" = "--stamp" ]; then
  log=$2; shift 2
  tmp=$(mktemp)
  { echo "# command: $*"; echo "# tree fingerprint $combined  [per-file list: fix-r2-fingerprint.txt]"; cat "$log"; } > "$tmp" && mv "$tmp" "$log"
else
  printf '%s\n# combined %s\n' "$fp" "$combined"
fi
