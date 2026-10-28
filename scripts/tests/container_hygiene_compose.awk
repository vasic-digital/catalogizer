# container_hygiene_compose.awk - compose-file reader for scripts/tests/test_container_hygiene.sh
#
# Purpose:  emit "service<TAB>kind<TAB>value" records for one compose file
#           (kinds: service build_ctx build_df image privileged userns volume health_test unmodeled).
#           The reader models block-style services at 2/4/6-space indentation, long/short/flow volumes,
#           and healthcheck tests (string, flow list, block list, block scalar). Anything inside `services:`
#           it recognises as unmodelled is emitted as `unmodeled`, reported as R10-COMPOSE-UNMODELED
#           (fail closed, §11.4.201): odd or unexpected indentation, non-plain top-level lines, aliases,
#           anchors, tags, merge keys, flow bodies it cannot split. It is a line reader, not a YAML parser:
#           shapes it does not recognise as odd can still be read wrongly (gap list in
#           docs/scripts/container_local.md, section "Hygiene guard parser coverage").
# Usage:    awk -f scripts/tests/container_hygiene_compose.awk docker-compose.yml
# Side-fx:  none (reads the file, writes records to stdout)
# Kept in its own file (not inline in the shell script) so it is read as awk, not as shell text.

function ind(s){ match(s,/^ */); return RLENGTH }
function unq(v){ sub(/^[ ]+/,"",v); sub(/[ ]+#.*$/,"",v); gsub(/^["\047]|["\047 ]+$/,"",v); return v }
function fget(b,k,   r){ if (match(b, "[{, ][\"\047]?" k "[\"\047]?[ ]*:[ ]*[^,}]*")) { r=substr(b,RSTART,RLENGTH); sub(/^[^:]*:[ ]*/,"",r); return unq(r) } return "" }
function vemit(ty,src,tgt,ro){
  if (ty=="volume" || ty=="tmpfs" || ty=="npipe" || ty=="image" || ty=="cluster") return
  if (src=="") return
  # a long-form bind source is a host path even without ./ (short form `svc:/x` would be a named volume)
  if (ty=="bind" && src !~ /^[.\/~$]/) src = "./" src
  # the target is not read by any rule; its colons would break the src:tgt:opts split
  gsub(/:/, "_", tgt)
  print svc "\tvolume\t" src ":" tgt ((ro ~ /^(true|yes|on|1)$/) ? ":ro" : "") }
function vflush(){ if (lv) { vemit(ltype,lsrc,ltgt,lro); lv=0 } }
function flush(){ vflush(); if (intest) { print svc "\thealth_test\t" tst; intest=0 } }
# flow-mapping service body: model image/privileged/userns_mode, fail closed on what it cannot model
function flowsvc(b,   v){
  v=fget(b,"privileged"); if (v!="") print svc "\tprivileged\t" v
  v=fget(b,"userns_mode"); if (v!="") print svc "\tuserns\t" v
  v=fget(b,"image"); if (v!="") print svc "\timage\t" v
  if (b ~ /(volumes|build|healthcheck|<<)["\047]?[ ]*:/ || b ~ /[*&!][A-Za-z0-9_!]/) print svc "\tunmodeled\tflow-style service body uses keys/anchors/tags this parser cannot model: " substr(b,1,60)
}
function um(m){ print (svc=="" ? "-" : svc) "\tunmodeled\t" m }
# odd(v): a value the parser does not model (alias, anchor, tag, block scalar, flow collection)
function odd(v){ return v ~ /^[*&!|>{\[]/ }
function bscal(v){ return v ~ /^[|>][-+0-9]*$/ }
function bal(s,   a,b,c,d){ a=gsub(/\{/,"{",s); b=gsub(/\}/,"}",s); c=gsub(/\[/,"[",s); d=gsub(/\]/,"]",s); return a==b && c==d }
# kv(s): split "key: value" (key may be quoted, space before the colon allowed) into K and V; 0 if not a key line
function kv(s){
  if (!match(s, /^[ ]*["\047]?[A-Za-z0-9_.<-]+["\047]?[ ]*:([ ]|$)/)) return 0
  K=substr(s, RSTART, RLENGTH); sub(/^[ ]*["\047]?/,"",K); sub(/["\047]?[ ]*:[ ]*$/,"",K)
  V=substr(s, RSTART+RLENGTH); sub(/^[ ]+/,"",V); sub(/[ ]+#.*$/,"",V); sub(/^#.*$/,"",V); sub(/[ ]+$/,"",V); return 1 }
function lset(k,v){ if (k=="type") ltype=v; else if (k=="source"||k=="src") lsrc=v; else if (k=="target"||k=="dst"||k=="destination") ltgt=v; else if (k=="read_only") lro=v }

BEGIN { bsind=-1 }
/^[ ]*#/ {next}
/^[ ]*$/ {next}
{ i=ind($0) }
# content of a block scalar: consumed raw (a test: block scalar is collected)
bsind>=0 && i>bsind { if (intest) { t=$0; sub(/^[ ]+/,"",t); tst = tst " " t }; next }
{ bsind=-1 }
# continuation lines of an unbalanced flow collection, whatever their indentation, until it closes
inflow { fbuf = fbuf " " $0; if (bal(fbuf)) inflow=0; next }
i==0 { flush(); insvc=($0 ~ /^services:/); svc=""; badsvc=0
  # top level: only a plain `key:` (optionally with a value) or a document marker is modelled
  if ($0 !~ /^[A-Za-z0-9_.-]+:([ ]|$)/ && $0 !~ /^(---|\.\.\.)[ ]*$/) { print "-\tunmodeled\ttop-level line the parser cannot model (quoted key, space before colon, JSON/flow document): " substr($0,1,60); insvc=0; next }
  if (insvc && $0 ~ /^services:[ ]*[^ #]/) print "-\tunmodeled\tflow-style services mapping on one line"
  next }
!insvc {next}
i==2 && /^  [A-Za-z0-9_.-]+:[ ]*(#.*)?$/ { flush(); svc=$1; sub(":","",svc); invol=0; inbuild=0; inhc=0; ck=""; badsvc=0; first=1; print svc "\tservice\t-"; next }
i==2 && /^  [A-Za-z0-9_.-]+:[ ]*\{/ { flush(); svc=$1; sub(":.*","",svc); invol=0; inbuild=0; inhc=0; ck=""; badsvc=0; first=0; print svc "\tservice\t-"
  t=$0; sub(/[ ]+#[^}\]]*$/,"",t)
  if (!bal(t)) { um("multi-line flow service body: " substr($0,1,60)); svc=""; badsvc=1; inflow=1; fbuf=t; next }
  flowsvc($0); next }
i==2 { flush(); svc=""; badsvc=1; print "-\tunmodeled\tservice entry the parser cannot model: " substr($0,1,60); next }
svc=="" { if (!badsvc) um("line under services: outside a modelled service (indentation not 2/4?): " substr($0,1,60)); badsvc=1; next }
first { first=0; if (i!=4) { um("service body not indented by 4 spaces"); svc=""; badsvc=1; next } }
i==1 || i==3 { flush(); um("line at an indentation the parser does not model: " substr($0,1,60)); next }
invol && lv && i>vind && kv($0) { lset(K, unq(V)); next }
/^[ ]*-([ ]|$)/ {
  item=$0; sub(/^[ ]*-[ ]?/,"",item); sub(/[ ]+#.*$/,"",item)
  # a block-scalar item (`- |`, `- key: >`): its deeper lines are scalar content, never structure
  if (bscal(item) || (kv(item) && bscal(V))) { bsind=i; if (invol) um("volume item is a block scalar"); else if (intest) tst=tst " "; next }
  if (i<=4) { if (ck=="volumes") um("volumes sequence at the key indentation"); next }
  if (invol) {
    vflush()
    if (item ~ /^[{\[]/) {
      if (!bal(item)) { um("multi-line flow volume item: " substr(item,1,60)); inflow=1; fbuf=item; next }
      if (item ~ /^\[/ || item ~ /[*&!][A-Za-z0-9_!]/) { um("volume item the parser cannot model: " substr(item,1,60)); next }
      vemit(fget(item,"type"), fget(item,"source") fget(item,"src"), fget(item,"target") fget(item,"dst") fget(item,"destination"), fget(item,"read_only"))
    }
    else if (item ~ /^[*&!|>]/) um("volume item the parser cannot model: " substr(item,1,60))
    else if (kv(item)) { lv=1; vind=i; ltype=""; lsrc=""; ltgt=""; lro=""; lset(K, unq(V)) }
    else print svc "\tvolume\t" unq(item)
  }
  else if (intest) tst = tst " " item
  next }
i==4 { flush(); invol=0; inbuild=0; inhc=0; ck=""; k6open=0
  if ($0 ~ /^    [{\[]/) { t=$0; sub(/[ ]+#[^}\]]*$/,"",t)
    if (!bal(t)) { um("multi-line flow service body: " substr($0,1,60)); inflow=1; fbuf=t; next }
    flowsvc($0); next }
  if (!kv($0)) { um("service line the parser cannot model: " substr($0,1,60)); next }
  ck=K
  if (K=="<<") { um("merge key (<<) in service: values come from an anchor the parser cannot see"); next }
  rel=(K ~ /^(privileged|userns_mode|build|volumes|healthcheck)$/)
  if (bscal(V)) { bsind=4; if (rel) um(K " value is a block scalar"); next }
  if ((K=="privileged" || K=="userns_mode") && V ~ /^["\047]?\$/) { um(K " value is env-substituted: " V); next }
  if (rel && odd(V)) { um(K " value is an alias/anchor/tag/flow collection: " substr(V,1,40)); if (!bal(V)) { inflow=1; fbuf=V }; next }
  if (K=="volumes") { invol=1 }
  else if (K=="healthcheck") { inhc=1; if (V!="") um("healthcheck value the parser cannot model: " V) }
  else if (K=="build") { if (V!="") { print svc "\tbuild_ctx\t" unq(V); print svc "\tbuild_df\tDockerfile" } else inbuild=1 }
  else if (K=="image") print svc "\timage\t" unq(V)
  else if (K=="privileged") print svc "\tprivileged\t" unq(V)
  else if (K=="userns_mode") print svc "\tuserns\t" unq(V)
  else if (V ~ /^[{\[]/ && !bal(V)) { inflow=1; fbuf=V }
  next }
i==6 { flush(); k6open=0
  if (!kv($0)) { if (inbuild || inhc || invol) um("line under build/healthcheck/volumes the parser cannot model: " substr($0,1,60)); next }
  if (invol) { um("volumes given as a mapping, not a sequence: " substr($0,1,60)); next }
  k6open=(V=="")
  if (bscal(V)) { bsind=6; if (inhc && K=="test") { intest=1; tst="" } else if (inbuild) um("build " K " is a block scalar"); next }
  if (inbuild && (K=="context" || K=="dockerfile")) {
    if (odd(V)) { um("build " K " value the parser cannot model: " V); next }
    print svc "\t" (K=="context" ? "build_ctx" : "build_df") "\t" unq(V) }
  else if (K=="test") {
    if (V=="") { intest=1; tst="" }
    else if (!bal(V)) { um("multi-line flow healthcheck test: " substr(V,1,60)); inflow=1; fbuf=V }
    else if (V ~ /^[*&!{]/) um("healthcheck test value the parser cannot model: " substr(V,1,60))
    else print svc "\thealth_test\t" V }
  else if (V ~ /^[{\[]/ && !bal(V)) { inflow=1; fbuf=V }
  next }
# catch-all inside a service: an odd indentation, or a line nested deeper than an open 6-space map key
# (a build/healthcheck/volumes body at 7 or 8, a sibling indented under a key that has a value) fails closed
{ if (i % 2 == 1) um("line at an odd indentation inside a service: " substr($0,1,60))
  else if (i > 6 && !k6open) um("line nested deeper than its parent key allows: " substr($0,1,60))
  next }
END { flush() }
