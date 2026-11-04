#!/usr/bin/env bash
# Adds ONE large video (~200 MB, 140 s, constant 12 Mb/s) to a media corpus produced by scripts/test-infra/gen_media_fixtures.sh and
# registers it in the corpus manifest, so the long-playback live spec has a file the browser cannot hold fully buffered (a seek far
# ahead is then genuinely served by the network and needs a live stream ticket).
# usage: gen_big_fixture.sh <corpus-dir>      idempotent; the file and the manifest entry are only written when missing/different.
set -euo pipefail
dir=${1:?corpus dir}; rel="movies/Big.Seek.Movie.2026.140s.mp4"; f="${dir:?}/$rel"; man="${dir:?}/manifest.json"
[ -f "$man" ] || { echo "no manifest.json in $dir" >&2; exit 2; }
if [ ! -f "$f" ]; then
  tmp=$(mktemp "${f%/*}/.big.XXXXXX.mp4")
  ffmpeg -nostdin -loglevel error -y -f lavfi -i "testsrc2=size=1280x720:rate=25:duration=140" -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=140" \
    -c:v libx264 -preset ultrafast -b:v 12M -minrate 12M -maxrate 12M -bufsize 24M -x264-params nal-hrd=cbr -pix_fmt yuv420p -c:a aac -b:a 96k -movflags +faststart "$tmp"
  chmod 0644 "$tmp"; mv -f -- "$tmp" "$f"
fi
python3 - "$f" "$man" "$rel" <<'P'
import json, hashlib, subprocess, sys
f, man, rel = sys.argv[1:4]
h = hashlib.sha256(open(f, 'rb').read()).hexdigest()
m = json.load(open(man))
if rel not in m['files']:
    pr = json.loads(subprocess.check_output(['ffprobe', '-v', 'error', '-show_format', '-show_streams', '-of', 'json', f]))
    import os
    size = os.path.getsize(f)
    m['files'][rel] = {'class': 'movies', 'duration_s': round(float(pr['format']['duration']), 2), 'format': pr['format']['format_name'], 'probe_ok': True,
                       'sha256': h, 'size': size, 'streams': [{'codec_type': s['codec_type'], 'codec_name': s['codec_name']} for s in pr['streams']]}
    m['file_count'] = len(m['files']); m['total_bytes'] = sum(v['size'] for v in m['files'].values())
    json.dump(m, open(man, 'w'), indent=1, sort_keys=True)
assert m['files'][rel]['sha256'] == h, 'manifest sha256 differs from the file'
print(rel, m['files'][rel]['size'], 'bytes registered; file_count', m['file_count'])
P
