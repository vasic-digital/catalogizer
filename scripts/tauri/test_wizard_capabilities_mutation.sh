#!/usr/bin/env bash
# Paired mutation test for scripts/tauri/test_wizard_capabilities.sh (TAURI-R1-FU-1/2).
# Builds a throw-away fake repo root (guard copy + installer-wizard/{src,src-tauri/capabilities}) under a mktemp dir, applies one
# mutation per case and asserts the guard's exit code. GUARD=<path> runs another guard copy against the same cases (RED proof).
# Exit codes: 0 all cases behaved as expected / 1 at least one case did not / 2 setup error.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
GUARD="${GUARD:-$HERE/test_wizard_capabilities.sh}"
[ -f "$GUARD" ] || { echo "setup: guard missing: $GUARD" >&2; exit 2; }
TMP="$(mktemp -d "${TMPDIR:-/tmp}/wizcap-mut.XXXXXX")" || { echo "setup: mktemp failed" >&2; exit 2; }
trap 'rm -rf "${TMP:?}"' EXIT
bad=0
run_case() { # name expected_rc mutate_fn
  local name="$1" want="$2" fn="$3" R="$TMP/${1}"
  mkdir -p "${R:?}/scripts/tauri" "$R/installer-wizard/src/services" "$R/installer-wizard/src-tauri/capabilities"
  { cp "$GUARD" "$R/scripts/tauri/test_wizard_capabilities.sh" \
    && cp "$ROOT/installer-wizard/src/services/tauri.ts" "$R/installer-wizard/src/services/tauri.ts" \
    && cp "$ROOT/installer-wizard/src-tauri/capabilities/default.json" "$R/installer-wizard/src-tauri/capabilities/default.json" \
    && cp "$ROOT/installer-wizard/src-tauri/tauri.conf.json" "$R/installer-wizard/src-tauri/tauri.conf.json"; } \
    || { echo "setup: copy failed" >&2; exit 2; }
  W="$R/installer-wizard"; "$fn"
  local rc=0; bash "$R/scripts/tauri/test_wizard_capabilities.sh" >/dev/null 2>&1 || rc=$?
  if [ "$rc" = "$want" ]; then echo "ok   $name (rc=$rc)"; else echo "BAD  $name: want rc=$want got rc=$rc"; bad=1; fi
}
CAP="src-tauri/capabilities"
m_baseline() { :; }
m_other_window() { printf '%s\n' '{"identifier":"default","windows":["other"],"permissions":["dialog:allow-open","dialog:allow-save"]}' >"$W/$CAP/default.json"
  printf '%s\n' '{"identifier":"empty-main","windows":["main"],"permissions":[]}' >"$W/$CAP/second.json"; }
m_platforms_macos() { printf '%s\n' '{"identifier":"default","windows":["main"],"platforms":["macOS"],"permissions":["dialog:allow-open","dialog:allow-save"]}' >"$W/$CAP/default.json"; }
m_split_caps() { printf '%s\n' '{"identifier":"a","windows":["main"],"permissions":["dialog:allow-open"]}' >"$W/$CAP/default.json"
  printf '%s\n' '{"identifier":"b","windows":["main"],"permissions":["dialog:allow-save"]}' >"$W/$CAP/second.json"; }
m_no_save() { printf '%s\n' '{"identifier":"default","windows":["main"],"permissions":["dialog:allow-open"]}' >"$W/$CAP/default.json"; }
m_no_main() { printf '%s\n' '{"identifier":"default","windows":["x"],"permissions":["dialog:allow-open","dialog:allow-save"]}' >"$W/$CAP/default.json"; }
m_linux_platform_ok() { printf '%s\n' '{"identifier":"default","windows":["main"],"platforms":["linux","macOS"],"permissions":["dialog:allow-open",{"identifier":"dialog:allow-save"}]}' >"$W/$CAP/default.json"; }
m_bad_json() { printf '%s\n' '{not json' >"$W/$CAP/default.json"; }
setsrc() { printf '%s\n' "$1" >"$W/src/services/tauri.ts"; }
# import-detection mutations: the permission set must follow the import style; capability grants only "open" in these cases
only_open() { printf '%s\n' '{"identifier":"default","windows":["main"],"permissions":["dialog:allow-open"]}' >"$W/$CAP/default.json"; }
m_dq_save() { only_open; setsrc 'import { save } from "@tauri-apps/plugin-dialog"
export const x = save'; }
m_multiline_save() { only_open; setsrc 'import {
  open,
  save,
} from '"'"'@tauri-apps/plugin-dialog'"'"''; }
m_star_import() { only_open; setsrc "import * as dlg from '@tauri-apps/plugin-dialog'"; }
m_dynamic_import() { only_open; setsrc "export const f = () => import('@tauri-apps/plugin-dialog')"; }
m_alias_save() { only_open; setsrc "import { save as saveDlg } from '@tauri-apps/plugin-dialog'"; }
m_blind() { setsrc "export const nothing = 1"; }
m_open_only_dq_ok() { only_open; setsrc 'import { open } from "@tauri-apps/plugin-dialog"'; }
cap() { printf '%s\n' "$1" >"$W/$CAP/default.json"; }
P='"permissions":["dialog:allow-open","dialog:allow-save"]'
m_windows_string_mainx() { cap '{"identifier":"d","windows":"mainx",'"$P"'}'; }
m_windows_string_main_substr() { cap '{"identifier":"d","windows":"domain",'"$P"'}'; }
m_windows_string_main_exact() { cap '{"identifier":"d","windows":"main",'"$P"'}'; }
m_platforms_string_substr() { cap '{"identifier":"d","windows":["main"],"platforms":"linux-not",'"$P"'}'; }
m_deny_save() { cap '{"identifier":"d","windows":["main"],"permissions":["dialog:allow-open","dialog:allow-save","dialog:deny-save"]}'; }
m_deny_other_file() { cap '{"identifier":"d","windows":["main"],'"$P"'}'; printf '%s\n' '{"identifier":"x","windows":["main"],"permissions":["dialog:deny-open"]}' >"$W/$CAP/deny.json"; }
m_local_false() { cap '{"identifier":"d","windows":["main"],"local":false,'"$P"'}'; }
m_remote_urls() { cap '{"identifier":"d","windows":["main"],"remote":{"urls":["https://evil.example/*"]},'"$P"'}'; }
m_glob_star_window() { cap '{"identifier":"d","windows":["*"],'"$P"'}'; }
m_dialog_default() { cap '{"identifier":"d","windows":["main"],"permissions":["dialog:default"]}'; }
m_raw_invoke() { only_open; setsrc "import { open } from '@tauri-apps/plugin-dialog'
export const s = () => invoke('plugin:dialog|save', {})"; }
m_raw_invoke_only() { only_open; setsrc "export const s = () => invoke('plugin:dialog|save', {})"; }
run_case baseline_pass 0 m_baseline
run_case other_window_plus_empty_main 1 m_other_window
run_case platforms_macos_only 1 m_platforms_macos
run_case perms_split_across_caps 1 m_split_caps
run_case save_missing 1 m_no_save
run_case no_main_window 1 m_no_main
run_case linux_in_platforms_and_object_perm_pass 0 m_linux_platform_ok
run_case invalid_json_fails 1 m_bad_json
run_case double_quote_import_save 1 m_dq_save
run_case multiline_import_save 1 m_multiline_save
run_case star_import 1 m_star_import
run_case dynamic_import 1 m_dynamic_import
run_case aliased_import_save 1 m_alias_save
run_case no_dialog_import_is_blind 2 m_blind
run_case open_only_double_quote_pass 0 m_open_only_dq_ok
run_case windows_string_mainx 1 m_windows_string_mainx
run_case windows_string_domain_substring 1 m_windows_string_main_substr
run_case windows_bare_string_main 1 m_windows_string_main_exact
run_case platforms_string_substring 1 m_platforms_string_substr
run_case deny_save_same_cap 1 m_deny_save
run_case deny_open_other_file 1 m_deny_other_file
run_case local_false 1 m_local_false
run_case remote_urls 1 m_remote_urls
run_case window_glob_star_pass 0 m_glob_star_window
run_case dialog_default_pass 0 m_dialog_default
run_case raw_invoke_plugin_dialog_save 1 m_raw_invoke
run_case raw_invoke_only_not_blind 1 m_raw_invoke_only
# --- TAURI-R2-FU-1/2: capability formats, tauri.conf.json, comments/strings, ask/confirm/message ---
CONF="src-tauri/tauri.conf.json"
setconf() { python3 -I -c 'import json,sys;p=sys.argv[1];d=json.load(open(p));exec(sys.argv[2]);json.dump(d,open(p,"w"))' "$W/$CONF" "$1"; }
m_toml_ok() { rm "$W/$CAP/default.json"; printf '%s\n' 'identifier = "default"' 'windows = ["main"]' 'permissions = ["dialog:allow-open", "dialog:allow-save"]' >"$W/$CAP/default.toml"; }
m_toml_missing_save() { rm "$W/$CAP/default.json"; printf '%s\n' 'identifier = "default"' 'windows = ["main"]' 'permissions = ["dialog:allow-open"]' >"$W/$CAP/default.toml"; }
m_toml_table_perms_ok() { rm "$W/$CAP/default.json"; printf '%s\n' 'identifier = "default"' 'windows = ["main"]' '[[permissions]]' 'identifier = "dialog:allow-open"' '[[permissions]]' 'identifier = "dialog:allow-save"' >"$W/$CAP/default.toml"; }
m_toml_bad() { rm "$W/$CAP/default.json"; printf '%s\n' 'identifier = = broken' >"$W/$CAP/default.toml"; }
m_json5_ok() { rm "$W/$CAP/default.json"; printf '%s\n' '// comment' '{ "identifier": "default", /* c */ "windows": ["main"], "permissions": ["dialog:allow-open", "dialog:allow-save",], }' >"$W/$CAP/default.json5"; }
m_json5_missing_save() { rm "$W/$CAP/default.json"; printf '%s\n' '{ "identifier": "default", "windows": ["main"], "permissions": ["dialog:allow-open",], }' >"$W/$CAP/default.json5"; }
m_json5_unparsable() { rm "$W/$CAP/default.json"; printf '%s\n' "{ identifier: 'default', windows: ['main'], permissions: ['dialog:allow-open','dialog:allow-save'] }" >"$W/$CAP/default.json5"; }
m_conf_lists_other() { setconf 'd["app"]["security"]["capabilities"]=["other"]'; }
m_conf_lists_default() { setconf 'd["app"]["security"]["capabilities"]=["default"]'; }
m_conf_inline_cap_ok() { rm "$W/$CAP/default.json"; setconf 'd["app"]["security"]["capabilities"]=[{"identifier":"inl","windows":["main"],"permissions":["dialog:allow-open","dialog:allow-save"]}]'; }
m_conf_label_wizard_cap_main() { setconf 'd["app"]["windows"][0]["label"]="wizard"'; }
m_conf_label_wizard_cap_wizard() { setconf 'd["app"]["windows"][0]["label"]="wizard"'; cap '{"identifier":"d","windows":["wizard"],'"$P"'}'; }
m_conf_bad() { printf '%s\n' '{nope' >"$W/$CONF"; }
m_comment_line_import() { only_open; setsrc "import { open } from '@tauri-apps/plugin-dialog'
// import { save } from '@tauri-apps/plugin-dialog'"; }
m_comment_block_import() { only_open; setsrc "import { open } from '@tauri-apps/plugin-dialog'
/* import { save } from '@tauri-apps/plugin-dialog' */"; }
m_string_import() { only_open; setsrc "import { open } from '@tauri-apps/plugin-dialog'
export const doc = \"import { save } from '@tauri-apps/plugin-dialog'\""; }
m_template_import() { only_open; setsrc "import { open } from '@tauri-apps/plugin-dialog'
export const doc = \`import { save } from '@tauri-apps/plugin-dialog'\`"; }
m_comment_only_is_blind() { only_open; setsrc "// import { open } from '@tauri-apps/plugin-dialog'"; }
m_real_import_after_comment() { only_open; setsrc "// header
import { open, save } from '@tauri-apps/plugin-dialog' // trailing"; }
m_ask_import() { only_open; setsrc "import { open, ask } from '@tauri-apps/plugin-dialog'"; }
m_confirm_import() { only_open; setsrc "import { open, confirm } from '@tauri-apps/plugin-dialog'"; }
m_message_import() { only_open; setsrc "import { open, message as msg } from '@tauri-apps/plugin-dialog'"; }
m_ask_granted() { cap '{"identifier":"d","windows":["main"],"permissions":["dialog:allow-open","dialog:allow-ask"]}'; setsrc "import { open, ask } from '@tauri-apps/plugin-dialog'"; }
m_raw_message() { only_open; setsrc "export const s = () => invoke('plugin:dialog|message', {})"; }
m_star_needs_ask() { cap '{"identifier":"d","windows":["main"],'"$P"'}'; setsrc "import * as dlg from '@tauri-apps/plugin-dialog'"; }
run_case toml_cap_pass 0 m_toml_ok
run_case toml_cap_missing_save 1 m_toml_missing_save
run_case toml_table_perms_pass 0 m_toml_table_perms_ok
run_case toml_invalid_fails 1 m_toml_bad
run_case json5_cap_pass 0 m_json5_ok
run_case json5_cap_missing_save 1 m_json5_missing_save
run_case json5_unparsable_fails_closed 1 m_json5_unparsable
run_case conf_enables_only_other 1 m_conf_lists_other
run_case conf_enables_default_pass 0 m_conf_lists_default
run_case conf_inline_capability_pass 0 m_conf_inline_cap_ok
run_case conf_window_label_wizard_cap_main 1 m_conf_label_wizard_cap_main
run_case conf_window_label_wizard_cap_wizard_pass 0 m_conf_label_wizard_cap_wizard
run_case conf_invalid_json_fails 1 m_conf_bad
run_case line_comment_import_ignored 0 m_comment_line_import
run_case block_comment_import_ignored 0 m_comment_block_import
run_case string_literal_import_ignored 0 m_string_import
run_case template_literal_import_ignored 0 m_template_import
run_case comment_only_import_is_blind 2 m_comment_only_is_blind
run_case real_import_after_comment_needs_save 1 m_real_import_after_comment
run_case ask_import_needs_allow_ask 1 m_ask_import
run_case confirm_import_needs_allow_confirm 1 m_confirm_import
run_case aliased_message_import_needs_allow_message 1 m_message_import
run_case ask_granted_pass 0 m_ask_granted
run_case raw_invoke_message 1 m_raw_message
run_case star_import_needs_all_five 1 m_star_needs_ask
[ "$bad" = 0 ] && echo "ALL CASES OK" || echo "MUTATION TEST FAILED"
exit "$bad"
