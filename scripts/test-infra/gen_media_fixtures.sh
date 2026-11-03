#!/usr/bin/env bash
# gen_media_fixtures.sh - deterministic media/document fixture corpus for the LIVE validation matrix (chunk L1).
# Guide: docs/scripts/gen_media_fixtures.md
#
#   gen_media_fixtures.sh --out DIR            generate the corpus into DIR (DIR must be OUTSIDE the repo; emptied only if absent/empty/a previous corpus, else REFUSED rc=2)
#   gen_media_fixtures.sh --out DIR --check    generate twice (DIR/run1, DIR/run2) and compare every sha256: exit 0 identical, 1 differs
#   gen_media_fixtures.sh --out DIR --verify   re-hash DIR against DIR/manifest.json (exit 1 on drift)
# Env: GMF_MAX_BYTES (default 314572800 = 300 MB cap), GMF_TEST_NONDET=1 (TEST ONLY: leaves a nondeterministic flag in - used to prove --check goes RED).
# Everything is synthetic (ffmpeg lavfi, python): no downloads, no mocks, no host fonts. Output layout: movies/ tv/ music/ books/ photos/ subtitles beside their video, negative/.
set -euo pipefail
export LC_ALL=C TZ=UTC
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
OUT=""; MODE=gen
while [ $# -gt 0 ]; do
  case "$1" in
    --out) [ $# -ge 2 ] && [ -n "$2" ] || { echo "gen_media_fixtures: --out needs a value" >&2; exit 2; }; OUT=$2; shift 2;;
    --check) MODE=check; shift;;
    --verify) MODE=verify; shift;;
    -h|--help) sed -n 2,9p "$0"; exit 0;;
    *) echo "gen_media_fixtures: unknown argument $1" >&2; exit 2;;
  esac
done
[ -n "$OUT" ] || OUT="${QA_WORK:+$QA_WORK/media}"
[ -n "$OUT" ] || { echo "gen_media_fixtures: REFUSED reason=no_output_dir (--out DIR or QA_WORK)" >&2; exit 2; }
case "$OUT" in /*) ;; *) echo "gen_media_fixtures: REFUSED reason=relative_out" >&2; exit 2;; esac
OUT="$(readlink -m -- "$OUT")"
case "$OUT" in "$ROOT"|"$ROOT"/*) echo "gen_media_fixtures: REFUSED reason=out_inside_repo $OUT" >&2; exit 2;; esac
[ "$(echo "$OUT" | tr -cd / | wc -c)" -ge 3 ] || { echo "gen_media_fixtures: REFUSED reason=out_too_shallow $OUT" >&2; exit 2; }
# ---- deletion guard (constitution 11.4.287): the generator empties its target, so the target must be provably ours ----
refuse() { echo "gen_media_fixtures: REFUSED reason=$1 $OUT" >&2; exit 2; }
SAFE_RM_LIB="$ROOT/constitution/scripts/lib/safe_rm.sh"
[ -r "$SAFE_RM_LIB" ] || refuse safe_rm_lib_missing
# shellcheck source=/dev/null
. "$SAFE_RM_LIB"
MARKER_SCHEMA="catalogizer.media-fixtures/1"
under() { case "$2" in "$1"|"$1"/*) return 0;; esac; [ "$1" = / ]; }   # under A B: B equals A or lives below A
homes() { local h; for h in "${HOME:-}" $(echo "${SAFE_RM_EXTRA_HOMES:-}" | tr ':' ' '); do [ -n "$h" ] && readlink -m -- "$h"; done; }
out_guard_home() {        # refuse $HOME itself, an extra home, or any ancestor of one
  local h; while IFS= read -r h; do under "$1" "$h" && refuse out_is_or_contains_home; done < <(homes); return 0; }
out_guard_ancestor() {    # refuse the repo root or any ancestor of it
  under "$1" "$ROOT" && refuse out_is_or_contains_repo; return 0; }
out_guard_policy() {      # delete only what is absent, empty, or a previous corpus (manifest.json carrying our schema)
  local d=$1
  [ -e "$d" ] || [ -L "$d" ] || return 0
  [ -d "$d" ] && [ ! -L "$d" ] || { OUT=$d; refuse out_not_a_plain_directory; }
  [ -z "$(ls -A -- "$d")" ] && return 0
  if [ -f "$d/manifest.json" ] && python3 -I -c 'import json,sys; sys.exit(0 if json.load(open(sys.argv[1])).get("schema")==sys.argv[2] else 1)' "$d/manifest.json" "$MARKER_SCHEMA" 2>/dev/null; then return 0; fi
  OUT=$d; refuse out_has_foreign_content_no_corpus_marker
}
if [ "$MODE" != verify ]; then
  out_guard_ancestor "$OUT"
  out_guard_home "$OUT"
fi
for t in ffmpeg ffprobe python3; do command -v "$t" >/dev/null || { echo "gen_media_fixtures: REFUSED reason=tool_missing $t" >&2; exit 3; }; done
MAX="${GMF_MAX_BYTES:-314572800}"

FF=(ffmpeg -hide_banner -loglevel error -y -nostdin -threads 1 -filter_threads 1 -filter_complex_threads 1)
BX=(-bitexact -fflags +bitexact -flags:v +bitexact -flags:a +bitexact -map_metadata -1 -map_chapters -1)
NONDET=()
if [ "${GMF_TEST_NONDET:-}" = 1 ]; then NONDET=(-metadata creation_time=now); echo "gen_media_fixtures: WARNING GMF_TEST_NONDET=1 leaves a nondeterministic flag in (test only)" >&2; fi
VSRC() { echo "testsrc=size=$1:rate=${3:-25}:duration=$2"; }
# GRAD <WxH> <c0> <c1> <seed>: a fully pinned gradient frame (the filter is random unless colours, geometry AND seed are all fixed)
GRAD() { local w=${1%x*} h=${1#*x}; echo "gradients=s=$1:n=2:c0=$2:c1=$3:x0=0:y0=0:x1=$w:y1=$h:seed=$4:d=1:r=1"; }
ASRC() { echo "sine=frequency=${2:-440}:sample_rate=48000:duration=$1"; }

gen() {
  local D=$1
  out_guard_policy "$D"
  if [ -d "$D" ] && [ -n "$(ls -A -- "$D")" ]; then safe_rm_rf "$D" || { OUT=$D; refuse safe_rm_refused; }; fi
  mkdir -p "$D"/{movies,tv,music,books,photos,negative}
  printf '{"schema": "%s", "stub": true}\n' "$MARKER_SCHEMA" > "$D/manifest.json"   # marker so an interrupted run can be regenerated
  local M="$D/movies" T="$D/tv" U="$D/music" B="$D/books" P="$D/photos" N="$D/negative"
  # ---- video (5 s, 640x360, 25 fps, 440 Hz tone, burned-in testsrc timestamp counter) ----
  local V5; V5=(-f lavfi -i "$(VSRC 640x360 5)" -f lavfi -i "$(ASRC 5)")
  "${FF[@]}" "${V5[@]}" "${BX[@]}" "${NONDET[@]}" -c:v libx264 -preset veryfast -crf 28 -pix_fmt yuv420p -g 25 -x264-params threads=1:sliced-threads=0 -c:a aac -b:a 64k -shortest -movflags +faststart "$M/Movie.Name.2020.1080p.mp4"
  mkdir -p "$T/Show Name/Season 01"
  "${FF[@]}" "${V5[@]}" "${BX[@]}" -c:v libx264 -preset veryfast -crf 28 -pix_fmt yuv420p -g 25 -x264-params threads=1:sliced-threads=0 -c:a aac -b:a 64k -shortest "$T/Show Name/Season 01/Show Name S01E02.mkv"
  "${FF[@]}" "${V5[@]}" "${BX[@]}" -c:v libvpx-vp9 -b:v 0 -crf 40 -deadline good -cpu-used 5 -row-mt 0 -pix_fmt yuv420p -g 25 -c:a libopus -b:a 48k -shortest "$M/Another.Movie.2019.vp9.webm"
  "${FF[@]}" "${V5[@]}" "${BX[@]}" -c:v libaom-av1 -crf 45 -b:v 0 -cpu-used 8 -row-mt 0 -g 25 -pix_fmt yuv420p -c:a libopus -b:a 48k -shortest "$M/Third.Movie.2021.av1.webm"
  "${FF[@]}" "${V5[@]}" "${BX[@]}" -c:v libx265 -preset veryfast -crf 30 -pix_fmt yuv420p -g 25 -tag:v hvc1 -x265-params pools=none:frame-threads=1:no-wpp=1:log-level=none -c:a aac -b:a 64k -shortest "$M/Hevc.Movie.2022.hevc.mp4"
  "${FF[@]}" -f lavfi -i "$(VSRC 1920x1080 3)" -f lavfi -i "$(ASRC 3)" "${BX[@]}" -c:v libx264 -preset veryfast -crf 32 -pix_fmt yuv420p -g 25 -x264-params threads=1:sliced-threads=0 -c:a aac -b:a 64k -shortest "$M/Full.HD.Movie.2023.1080p.mp4"
  "${FF[@]}" -f lavfi -i "$(VSRC 3840x2160 1)" "${BX[@]}" -an -c:v libx264 -preset veryfast -crf 36 -pix_fmt yuv420p -g 25 -x264-params threads=1:sliced-threads=0 "$M/Ultra.HD.Movie.2024.2160p.mp4"
  "${FF[@]}" -f lavfi -i "$(VSRC 640x360 60)" -f lavfi -i "$(ASRC 60)" "${BX[@]}" -c:v libx264 -preset veryfast -crf 30 -pix_fmt yuv420p -g 50 -x264-params threads=1:sliced-threads=0 -c:a aac -b:a 48k -shortest -movflags +faststart "$M/Long.Seek.Movie.2018.60s.mp4"
  # ---- audio (3 s, 440 Hz) + silent control + tagged mp3 with cover ----
  local A3; A3=(-f lavfi -i "$(ASRC 3)")
  mkdir -p "$U/Test Artist - Test Album (2020)" "$U/Misc"
  "${FF[@]}" "${A3[@]}" "${BX[@]}" -c:a libmp3lame -b:a 96k -write_xing 0 -id3v2_version 0 "$U/Misc/Plain Tone.mp3"
  "${FF[@]}" "${A3[@]}" "${BX[@]}" -c:a flac "$U/Misc/Plain Tone.flac"
  "${FF[@]}" "${A3[@]}" "${BX[@]}" -c:a libvorbis -q:a 2 "$U/Misc/Plain Tone.ogg"
  "${FF[@]}" "${A3[@]}" "${BX[@]}" -c:a libopus -b:a 48k "$U/Misc/Plain Tone.opus"
  "${FF[@]}" "${A3[@]}" "${BX[@]}" -c:a pcm_s16le "$U/Misc/Plain Tone.wav"
  "${FF[@]}" "${A3[@]}" "${BX[@]}" -c:a aac -b:a 64k -f ipod "$U/Misc/Plain Tone.m4a"
  "${FF[@]}" -f lavfi -i "anullsrc=r=48000:cl=mono" -t 3 "${BX[@]}" -c:a pcm_s16le "$U/Misc/Silent Control.wav"
  "${FF[@]}" -f lavfi -i "$(GRAD 300x300 0x884422 0x22aa88 11)" -frames:v 1 "${BX[@]}" "$D/.cover.png"
  "${FF[@]}" "${A3[@]}" -i "$D/.cover.png" -map 0:a -map 1:v "${BX[@]}" -c:a libmp3lame -b:a 96k -write_xing 0 -c:v png -id3v2_version 3 \
     -metadata artist="Test Artist" -metadata album="Test Album" -metadata title="First Track" -metadata track=1 -metadata date=2020 -metadata:s:v title="Cover" -metadata:s:v comment="Cover (front)" \
     "$U/Test Artist - Test Album (2020)/01 - First Track.mp3"
  rm -f -- "$D/.cover.png"
  # ---- images ----
  local IMG=(-f lavfi -i "$(GRAD 320x240 0x336699 0xffcc00 7)" -frames:v 1)
  "${FF[@]}" "${IMG[@]}" "${BX[@]}" "$P/Color Bars Gradient.png"
  "${FF[@]}" "${IMG[@]}" "${BX[@]}" -pix_fmt yuvj420p -q:v 3 "$P/Color Bars Gradient.jpg"
  "${FF[@]}" "${IMG[@]}" "${BX[@]}" -c:v libwebp -quality 75 -compression_level 4 "$P/Color Bars Gradient.webp"
  "${FF[@]}" -f lavfi -i "testsrc2=size=160x120:rate=5:duration=2" "${BX[@]}" -loop 0 "$P/Animated Test.gif"
  "${FF[@]}" -f lavfi -i "color=c=0x336699:s=6000x6000:d=1:r=1" -frames:v 1 "${BX[@]}" "$P/Huge Dimension Small Bytes.png"
  # ---- python writer: svg, pdf, epub, cbz, subtitles, txt, negatives ----
  mkdir -p "$D/.cbz"
  for i in 1 2 3; do "${FF[@]}" -f lavfi -i "$(GRAD 300x450 0x$((i*3))0$((i*2))0$((i+4))0 0xeeddcc $((20+i)))" -frames:v 1 "${BX[@]}" "$D/.cbz/page$i.png"; done
  python3 -I - "$D" <<'PY'
import sys, os, zipfile
D = sys.argv[1]
def w(rel, data, mode="wb"):
    p = os.path.join(D, rel); os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, mode) as f: f.write(data)
# svg
w("photos/Test Vector.svg", '<?xml version="1.0" encoding="UTF-8"?>\n<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100" viewBox="0 0 200 100"><rect width="200" height="100" fill="#336699"/><circle cx="100" cy="50" r="30" fill="#ffcc00"/><text x="10" y="95" font-size="10" fill="#fff">catalogizer-svg-fixture</text></svg>\n', "w")
# pdf: 3 pages, Helvetica, known text, hand-built (deterministic, no deps)
PAGES = ["Catalogizer PDF fixture page one", "Second page carries the marker ZEBRA-4242", "Third and final page of the fixture"]
objs = []
def add(b): objs.append(b); return len(objs)
cat = add(None); pages = add(None); font = add(b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
kids = []
for t in PAGES:
    stream = ("BT /F1 24 Tf 72 700 Td (%s) Tj ET" % t).encode()
    c = add(b"<< /Length %d >>\nstream\n" % len(stream) + stream + b"\nendstream")
    pg = add(("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> >> >>" % (pages, c, font)).encode())
    kids.append(pg)
objs[cat-1] = ("<< /Type /Catalog /Pages %d 0 R >>" % pages).encode()
objs[pages-1] = ("<< /Type /Pages /Count %d /Kids [%s] >>" % (len(kids), " ".join("%d 0 R" % k for k in kids))).encode()
out = bytearray(b"%PDF-1.4\n"); offs = []
for i, o in enumerate(objs, 1):
    offs.append(len(out)); out += b"%d 0 obj\n" % i + o + b"\nendobj\n"
xr = len(out); out += b"xref\n0 %d\n0000000000 65535 f \n" % (len(objs)+1)
for o in offs: out += b"%010d 00000 n \n" % o
out += b"trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n" % (len(objs)+1, cat, xr)
w("books/Known Text Document.pdf", bytes(out))
def zinfo(name, stored=False):
    zi = zipfile.ZipInfo(name, date_time=(2020,1,1,0,0,0)); zi.external_attr = 0o644 << 16
    zi.compress_type = zipfile.ZIP_STORED if stored else zipfile.ZIP_DEFLATED; return zi
# cbz
with zipfile.ZipFile(os.path.join(D, "books/Test Comic 001.cbz"), "w") as z:
    for i in (1,2,3):
        z.writestr(zinfo("page%d.png" % i, True), open(os.path.join(D, ".cbz/page%d.png" % i), "rb").read())
# epub
CH = {"chapter1.xhtml": "Chapter One of the Catalogizer epub fixture.", "chapter2.xhtml": "Chapter Two carries the marker QUOKKA-7777."}
with zipfile.ZipFile(os.path.join(D, "books/Test Book.epub"), "w") as z:
    z.writestr(zinfo("mimetype", True), "application/epub+zip")
    z.writestr(zinfo("META-INF/container.xml"), '<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
    z.writestr(zinfo("OEBPS/content.opf"), '<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">urn:uuid:00000000-0000-4000-8000-000000000001</dc:identifier><dc:title>Test Book</dc:title><dc:creator>Test Author</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2020-01-01T00:00:00Z</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="c1" href="chapter1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="chapter2.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>')
    z.writestr(zinfo("OEBPS/nav.xhtml"), '<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Nav</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml">One</a></li><li><a href="chapter2.xhtml">Two</a></li></ol></nav></body></html>')
    for n, t in CH.items():
        z.writestr(zinfo("OEBPS/" + n), '<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>%s</title></head><body><p>%s</p></body></html>' % (n, t))
# subtitles: 3 cues matching the 5 s movie timing, plus an out-of-sync copy
CUES = [(0.5, 1.5, "First cue: hello catalogizer"), (2.0, 3.0, "Second cue: subtitle sync marker"), (3.5, 4.5, "Third cue: goodbye")]
def ts(s, sep): ms = int(round(s*1000)); return "%02d:%02d:%02d%s%03d" % (ms//3600000, ms//60000%60, ms//1000%60, sep, ms%1000)
def srt(shift=0.0): return "".join("%d\n%s --> %s\n%s\n\n" % (i+1, ts(a+shift, ","), ts(b+shift, ","), t) for i,(a,b,t) in enumerate(CUES))
base = "movies/Movie.Name.2020.1080p"
w(base + ".en.srt", srt(), "w")
w(base + ".en.vtt", "WEBVTT\n\n" + "".join("%s --> %s\n%s\n\n" % (ts(a,"."), ts(b,"."), t) for a,b,t in CUES), "w")
ass_t = lambda s: "%d:%02d:%02d.%02d" % (int(s)//3600, int(s)//60%60, int(s)%60, int(round((s-int(s))*100)))
w(base + ".en.ass", "[Script Info]\nScriptType: v4.00+\nPlayResX: 640\nPlayResY: 360\n\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\nStyle: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,10,10,10,1\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" + "".join("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n" % (ass_t(a), ass_t(b), t) for a,b,t in CUES), "w")
w("movies/Movie.Name.2020.1080p.out-of-sync.srt", srt(7.0), "w")
w("books/Plain Notes.txt", "Catalogizer plain text fixture.\nLine two with marker OKAPI-1357.\n", "w")
# negatives
w("negative/zero-byte.mp4", b"")
w("negative/corrupt-garbage.mp3", bytes((i*131+17) % 256 for i in range(4096)))
w("negative/not-a-pdf.pdf", b"%PDF-1.4\nthis is truncated garbage\n")
src = open(os.path.join(D, "movies/Movie.Name.2020.1080p.mp4"), "rb").read()
w("negative/truncated-tail.mp4", src[: len(src)//2])
src = open(os.path.join(D, "photos/Color Bars Gradient.png"), "rb").read()
w("negative/truncated.png", src[:200])
PY
  safe_rm_rf "$D/.cbz"
}

manifest() {
  python3 -I - "$1" <<'PY'
import sys, os, json, hashlib, subprocess
D = sys.argv[1]
TEXT = {"books/Known Text Document.pdf": {"pages": 3, "text": ["Catalogizer PDF fixture page one", "Second page carries the marker ZEBRA-4242", "Third and final page of the fixture"]},
        "books/Test Book.epub": {"text": ["Chapter One of the Catalogizer epub fixture.", "Chapter Two carries the marker QUOKKA-7777."], "title": "Test Book", "author": "Test Author"},
        "books/Test Comic 001.cbz": {"pages": 3}, "books/Plain Notes.txt": {"text": ["Catalogizer plain text fixture.", "Line two with marker OKAPI-1357."]},
        "photos/Test Vector.svg": {"text": ["catalogizer-svg-fixture"]}}
CUES = ["First cue: hello catalogizer", "Second cue: subtitle sync marker", "Third cue: goodbye"]
tags = {"music/Test Artist - Test Album (2020)/01 - First Track.mp3": {"artist": "Test Artist", "album": "Test Album", "title": "First Track", "track": "1", "date": "2020", "cover": True}}
ents = {}
for dp, dn, fn in os.walk(D):
    dn.sort()
    for f in sorted(fn):
        p = os.path.join(dp, f); rel = os.path.relpath(p, D)
        if rel == "manifest.json": continue
        h = hashlib.sha256(open(p, "rb").read()).hexdigest()
        e = {"sha256": h, "size": os.path.getsize(p)}
        top = rel.split("/")[0]; e["class"] = "negative" if top == "negative" else top
        ext = f.rsplit(".", 1)[-1].lower()
        if ext in ("mp4","mkv","webm","mp3","flac","ogg","opus","wav","m4a","png","jpg","webp","gif"):
            r = subprocess.run(["ffprobe","-v","error","-show_entries","stream=codec_type,codec_name,width,height,r_frame_rate,sample_rate,channels:format=format_name,duration","-of","json",p],capture_output=True,text=True)
            if r.returncode == 0 and r.stdout.strip():
                j = json.loads(r.stdout)
                e["probe_ok"] = True; e["format"] = j["format"]["format_name"]
                if "duration" in j["format"]: e["duration_s"] = round(float(j["format"]["duration"]), 2)
                e["streams"] = [{k: s[k] for k in ("codec_type","codec_name","width","height","r_frame_rate","sample_rate","channels") if k in s} for s in j.get("streams", [])]
            else: e["probe_ok"] = False
        if rel in TEXT: e["expected"] = TEXT[rel]
        if ext in ("srt","vtt","ass") and "out-of-sync" not in f: e["expected"] = {"cues": CUES, "cue_count": 3, "start_s": [0.5, 2.0, 3.5]}
        if "out-of-sync" in f: e["expected"] = {"cues": CUES, "cue_count": 3, "start_s": [7.5, 9.0, 10.5], "note": "shifted +7 s: sync verification must flag it"}
        if rel in tags: e["expected"] = tags[rel]
        if rel == "negative/truncated-tail.mp4":
            e["class"] = "partial_playback"
            e["expected"] = {"must_fail_playback_or_parse": False, "partial_playback": True, "note": "first half of Movie.Name.2020.1080p.mp4 (moov atom lost with faststart): demuxable/decodable prefix may or may not probe; consumers must not treat it as a hard parse failure"}
        elif rel.startswith("negative/"): e["expected"] = {"must_fail_playback_or_parse": True}
        if rel == "music/Misc/Silent Control.wav": e["expected"] = {"silent": True, "note": "RMS oracle negative control"}
        if rel.startswith("music/Misc/Plain Tone") or rel.endswith("First Track.mp3"): e.setdefault("expected", {}).update({"tone_hz": 440})
        if (top in ("movies", "tv")) and ext in ("mp4", "mkv", "webm") and "Ultra.HD" not in f:
            e.setdefault("expected", {}).update({"tone_hz": 440, "burned_in": "lavfi testsrc timestamp counter"})
        ents[rel] = e
tot = sum(e["size"] for e in ents.values())
print(json.dumps({"schema": "catalogizer.media-fixtures/1", "generator": "scripts/test-infra/gen_media_fixtures.sh", "file_count": len(ents), "total_bytes": tot, "files": ents}, indent=1, sort_keys=True))
PY
}

size_of() { du -sb -- "$1" | cut -f1; }
case "$MODE" in
  gen)
    gen "$OUT"; manifest "$OUT" > "$OUT/manifest.json"
    SZ=$(size_of "$OUT"); [ "$SZ" -le "$MAX" ] || { echo "gen_media_fixtures: FAIL reason=corpus_over_cap bytes=$SZ cap=$MAX" >&2; exit 1; }
    echo "gen_media_fixtures: OK dir=$OUT files=$(find "$OUT" -type f | wc -l) bytes=$SZ manifest=$OUT/manifest.json manifest_sha256=$(sha256sum "$OUT/manifest.json" | cut -d' ' -f1)";;
  check)
    mkdir -p "$OUT"; R1="$OUT/run1"; R2="$OUT/run2"
    gen "$R1"; manifest "$R1" > "$R1/manifest.json"
    SZ=$(size_of "$R1"); [ "$SZ" -le "$MAX" ] || { echo "gen_media_fixtures: FAIL reason=corpus_over_cap bytes=$SZ cap=$MAX" >&2; exit 1; }
    sleep 2; gen "$R2"; manifest "$R2" > "$R2/manifest.json"
    diffs=0
    while IFS= read -r l; do echo "NONDETERMINISTIC: $l"; diffs=$((diffs+1)); done < <(diff <(cd "$R1" && find . -type f | sort | xargs -d '\n' sha256sum) <(cd "$R2" && find . -type f | sort | xargs -d '\n' sha256sum) | grep '^[<>]' || true)
    n=$(find "$R1" -type f | wc -l)
    if [ "$diffs" -eq 0 ]; then echo "DETERMINISM PASS: files=$n bytes=$(size_of "$R1") run1_manifest=$(sha256sum "$R1/manifest.json" | cut -d' ' -f1) run2_manifest=$(sha256sum "$R2/manifest.json" | cut -d' ' -f1)"; exit 0
    else echo "DETERMINISM FAIL: $diffs differing hash lines over $n files"; exit 1; fi;;
  verify)
    [ -f "$OUT/manifest.json" ] || { echo "gen_media_fixtures: no manifest in $OUT" >&2; exit 2; }
    python3 -I - "$OUT" <<'PY'
import sys, json, hashlib, os
D = sys.argv[1]; m = json.load(open(os.path.join(D, "manifest.json"))); bad = 0
for rel, e in m["files"].items():
    p = os.path.join(D, rel)
    if not os.path.exists(p) or hashlib.sha256(open(p, "rb").read()).hexdigest() != e["sha256"]: print("DRIFT", rel); bad += 1
have = set()
for dp, dn, fn in os.walk(D):
    for f in fn: have.add(os.path.relpath(os.path.join(dp, f), D))
for rel in sorted(have - set(m["files"]) - {"manifest.json"}): print("EXTRA", rel); bad += 1
print("verify: %d files, %d drift/extra" % (len(m["files"]), bad)); sys.exit(1 if bad else 0)
PY
    ;;
esac
