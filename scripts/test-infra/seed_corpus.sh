#!/usr/bin/env bash
# seed_corpus.sh - T130. Deterministic corpus seeder of the WP-13 real-service stack (docs/16 section 10.4).
# Builds a fixed corpus of real file types (png, jpg, gif, mp3, flac, mkv, mp4, pdf, zip, exe, dmg, txt), unicode names, one long path and empty
# directories from a recorded seed value, with fixed modes and mtimes: two runs with the same seed give a byte-identical tree and the same digest.
# The byte streams come from sha256 in counter mode, never from a pseudo-random generator whose output depends on the interpreter version.
# Usage:  seed_corpus.sh --seed <string> --out <dir> [--checksum-file <file>]
#   --seed STR            seed value (recorded in the evidence); a different seed gives a different digest
#   --out DIR             corpus root to create; refused when it exists and is not empty (exit 2)
#   --checksum-file FILE  receives one line `<sha256>  corpus seed=<seed>`; default <out>.sha256
# Digest: sha256 over the lines `<relpath NFC>\0<dir|file>\0<size>\0<sha256 of content or empty>\n`, sorted by path, for every entry of the tree.
# Stdout: `corpus_sha256=<hex>` and `files=<n>`. Exit: 0 done; 2 usage / out not empty; 1 failure.
# Runs inside IMG-TESTUTIL (python3 3.11) through `scripts/test-in-container.sh tooling unit -- bash scripts/test-infra/seed_corpus.sh ...`;
# it needs only bash and python3, so it is also the same program everywhere. Documented in docs/scripts/seed_corpus.md.
set -u
SEED=""; OUT=""; SUMF=""
usage() { echo "seed_corpus: $1" >&2; echo "usage: seed_corpus.sh --seed <string> --out <dir> [--checksum-file <file>]" >&2; exit 2; }
while [ $# -gt 0 ]; do
  case "$1" in
    --seed) [ $# -ge 2 ] || usage "--seed needs a value"; SEED=$2; shift 2;;
    --out) [ $# -ge 2 ] || usage "--out needs a value"; OUT=$2; shift 2;;
    --checksum-file) [ $# -ge 2 ] || usage "--checksum-file needs a value"; SUMF=$2; shift 2;;
    *) usage "unknown argument '$1'";;
  esac
done
[ -n "$SEED" ] || usage "--seed is required"
[ -n "$OUT" ] || usage "--out is required"
[ -n "$SUMF" ] || SUMF="$OUT.sha256"
if [ -e "$OUT" ] && [ -n "$(ls -A "$OUT" 2>/dev/null)" ]; then usage "--out '$OUT' exists and is not empty"; fi
command -v python3 >/dev/null 2>&1 || { echo "seed_corpus: python3 is required" >&2; exit 1; }
# the load-bearing switches (the paired mutations of tests/infra/test_seed_corpus.sh change exactly one of these lines)
SEED_ENTROPY=""
SEED_FOR_STREAM="$SEED"
UNICODE_NAMES=1
LONG_PATH=1
PNG_MAGIC=1
DIGEST_REAL=1
export SEED_ENTROPY SEED_FOR_STREAM UNICODE_NAMES LONG_PATH PNG_MAGIC DIGEST_REAL SEED_RECORDED="$SEED"
mkdir -p "$OUT" || exit 1
exec python3 -I - "$OUT" "$SUMF" <<'PY'
import hashlib, os, struct, sys, unicodedata, zipfile, zlib, io
out, sumf = sys.argv[1], sys.argv[2]
E = os.environ
seed_stream, entropy = E["SEED_FOR_STREAM"], E["SEED_ENTROPY"]
unicode_names, long_path, png_magic, digest_real = E["UNICODE_NAMES"] == "1", E["LONG_PATH"] == "1", E["PNG_MAGIC"] == "1", E["DIGEST_REAL"] == "1"
EPOCH = 1700000000

def stream(label, n):
    """n deterministic bytes: sha256 in counter mode over (seed, entropy, label)."""
    b, c = bytearray(), 0
    while len(b) < n:
        b += hashlib.sha256(("%s\0%s\0%s\0%d" % (seed_stream, entropy, label, c)).encode("utf-8")).digest(); c += 1
    return bytes(b[:n])

def png(label, w=8, h=8):
    raw = b"".join(b"\x00" + stream(label + str(y), w * 3) for y in range(h))
    def chunk(t, d): return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d) & 0xffffffff)
    sig = b"\x89PNG\r\n\x1a\n" if png_magic else b"\x89PNX\r\n\x1a\n"
    return sig + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")
def jpg(label): return b"\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00" + stream(label, 6000) + b"\xff\xd9"
def gif(label): return b"GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"
def mp3(label): return b"ID3\x03\x00\x00\x00\x00\x00\x00" + stream(label, 20000)
def flac(label): return b"fLaC\x00\x00\x00\x22" + stream(label, 20000)
def mkv(label): return b"\x1a\x45\xdf\xa3\x9f\x42\x86\x81\x01" + stream(label, 30000)
def mp4(label): return b"\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom" + stream(label, 30000)
def pdf(label):
    body = "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>endobj\n"
    return body.encode() + b"%" + stream(label, 3000).hex().encode() + b"\n%%EOF\n"
def zipf(label):
    bio = io.BytesIO()
    with zipfile.ZipFile(bio, "w", zipfile.ZIP_STORED) as z:
        for i in range(3):
            zi = zipfile.ZipInfo("part%d.bin" % i, date_time=(2020, 1, 1, 0, 0, 0)); zi.external_attr = 0o644 << 16
            z.writestr(zi, stream(label + str(i), 2000))
    return bio.getvalue()
def exe(label): return b"MZ\x90\x00" + stream(label, 8000)
def dmg(label): return b"x\x01\x73\x0d\x62\x62\x60\x60" + stream(label, 8000)
def txt(label): return (label + "\n").encode("utf-8") + stream(label, 500).hex().encode() + b"\n"

L, U = "x" * 0, ""
files = [
 ("movies/The.Matrix.1999.1080p.BluRay.mkv", mkv), ("movies/Inception.2010.720p.BluRay.mp4", mp4), ("movies/Interstellar.2014.2160p.UHD.mkv", mkv),
 ("music/Artist - Song Title.mp3", mp3), ("music/Another Artist - Track Name.flac", flac),
 ("music/Pink Floyd/The Dark Side of the Moon/01 - Speak to Me.flac", flac), ("music/Pink Floyd/The Dark Side of the Moon/02 - Breathe.flac", flac),
 ("series/Breaking Bad/Season 01/Breaking.Bad.S01E01.Pilot.mkv", mkv), ("series/Breaking Bad/Season 01/Breaking.Bad.S01E02.mkv", mkv),
 ("series/Breaking Bad/Season 02/Breaking.Bad.S02E01.mkv", mkv),
 ("software/app-installer-v1.0.exe", exe), ("software/tool-setup-2.3.1.dmg", dmg),
 ("pictures/cover-front.png", png), ("pictures/cover-back.jpg", jpg), ("pictures/spinner.gif", gif),
 ("docs/manual.pdf", pdf), ("docs/readme.txt", txt), ("docs/archive.zip", zipf),
 ("deep/level1/level2/level3/level4/deep_file.txt", txt),
]
if unicode_names:
    files += [("music/Björk - Jóga (Ünicöde).flac", flac), ("movies/千と千尋の神隠し.2001.mkv", mkv),
              ("docs/Привет мир.txt", txt), ("series/Ünicödé Show/Séason 01/E01 — Café.mp4", mp4)]
else:
    files += [("music/Bjork - Joga.flac", flac), ("movies/Spirited.Away.2001.mkv", mkv), ("docs/Privet mir.txt", txt), ("series/Unicode Show/Season 01/E01.mp4", mp4)]
if long_path:
    seg = lambda c: (c * 80)
    files.append((seg("a") + "/" + seg("b") + "/" + seg("c") + "/" + ("long-name-" + "n" * 100 + ".txt"), txt))
else:
    files.append(("short/long-name.txt", txt))
dirs_empty = ["empty", "deep/empty-leaf"]

for rel, fn in files:
    p = os.path.join(out, rel); os.makedirs(os.path.dirname(p), exist_ok=True)
    data = fn(rel)
    with open(p, "wb") as f: f.write(data)
    os.chmod(p, 0o644)
for rel in dirs_empty: os.makedirs(os.path.join(out, rel), exist_ok=True)
entries = []
for d, ds, fs in os.walk(out):
    for n in ds: entries.append(os.path.join(d, n))
    for n in fs: entries.append(os.path.join(d, n))
for p in entries:
    if os.path.isdir(p): os.chmod(p, 0o755)
for p in sorted(entries, key=len, reverse=True): os.utime(p, (EPOCH, EPOCH))
os.chmod(out, 0o755); os.utime(out, (EPOCH, EPOCH))

lines = []
for p in entries:
    rel = unicodedata.normalize("NFC", os.path.relpath(p, out))
    if os.path.isdir(p): lines.append((rel, "dir", 0, ""))
    else: lines.append((rel, "file", os.path.getsize(p), hashlib.sha256(open(p, "rb").read()).hexdigest()))
lines.sort()
m = hashlib.sha256()
for r, k, s, h in lines: m.update(("%s\0%s\0%d\0%s\n" % (r, k, s, h)).encode("utf-8"))
digest = m.hexdigest() if digest_real else "0" * 64
with open(sumf, "w", encoding="utf-8") as f: f.write("%s  corpus seed=%s\n" % (digest, E["SEED_RECORDED"]))
print("corpus_sha256=" + digest); print("files=%d" % sum(1 for l in lines if l[1] == "file"))
PY
