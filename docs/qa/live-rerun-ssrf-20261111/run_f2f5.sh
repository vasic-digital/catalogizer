#!/bin/bash
# SSRF-F2..F5 live probe: runs the DEPLOYED ffprobe/ffmpeg + the deployed binary inside catqa3-api (no restart, no prod edit).
# Guard args are copied from catalog-api/internal/services/cover_art_service.go (coverArtDemuxers, ffmpegInputGuard, ffprobeDurationCmdContext).
# usage: run_f2f5.sh RUN#
set -u
N=${1:?}; PORT=$((47940+N)); LAN=169.254.1.2; C=catqa3-api
HL=$(mktemp); LP=
python3 "$(dirname "$0")/listener.py" 0.0.0.0 "$HL" $PORT & LP=$!
D=/tmp/ssrf_f_$$_$N
trap 'kill $LP 2>/dev/null; rm -f "${HL:?}"; podman exec $C rm -rf "${D:?}" 2>/dev/null' EXIT
sleep 1
DEM="mov,mp4,m4a,3gp,3g2,mj2,matroska,webm,avi,asf,flv,mpegts,mpeg,mpegvideo,ogg,wav,flac,mp3,aac,ac3,eac3,dts,ape,wv,aiff,caf,amr,au,w64,rm,h264,hevc,m4v,ivf,obu"
GP="-protocol_whitelist file,pipe -format_whitelist $DEM"
hits(){ grep -c HIT "$HL"; }
x(){ podman exec $C sh -c "$1"; }
x "mkdir -p $D && cd $D && ffmpeg -nostdin -v error -f lavfi -i testsrc=d=2:s=64x64:r=10 -y real.mp4 && printf '#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nfile:///$D/real.mp4\n#EXT-X-ENDLIST\n' > pl.m3u8 && printf '#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nhttp://$LAN:$PORT/seg.ts\n#EXT-X-ENDLIST\n' > net.m3u8" ; echo "setup rc=$?"
echo "control: listener reachable from container: $(x "wget -q -T 5 -O - http://$LAN:$PORT/ctl" | head -c 5) hits=$(hits)"
: > "$HL"
echo "== F5 playlist->local file (control = real.mp4 directly must give a duration)"
echo "ctl ffprobe guarded real.mp4 -> $(x "cd $D; ffprobe -v error $GP -show_entries format=duration -of default=nw=1:nk=1 real.mp4" 2>&1 | head -c 60)"
echo "unguarded ffprobe pl.m3u8 (RED: expect duration) -> $(x "cd $D; ffprobe -v error -show_entries format=duration -of default=nw=1:nk=1 pl.m3u8" 2>&1 | head -c 60)"
echo "GUARDED ffprobe pl.m3u8 (expect refused, no duration) -> $(x "cd $D; ffprobe -v error $GP -show_entries format=duration -of default=nw=1:nk=1 pl.m3u8; echo rc=\$?" 2>&1 | tr '\n' ' ' | head -c 160)"
x "cd $D; ffmpeg -nostdin -protocol_whitelist file,pipe -format_whitelist $DEM -ss 0.5 -i pl.m3u8 -vframes 1 -y out_g.jpg >/dev/null 2>&1; echo -n"; echo "GUARDED ffmpeg pl.m3u8 thumbnail produced? $(x "test -s $D/out_g.jpg && echo YES || echo NO")"
x "cd $D; ffmpeg -nostdin -ss 0.5 -i pl.m3u8 -vframes 1 -y out_u.jpg >/dev/null 2>&1; echo -n"; echo "UNGUARDED ffmpeg pl.m3u8 thumbnail produced? (RED) $(x "test -s $D/out_u.jpg && echo YES || echo NO")"
echo "== F3/F5 network reach"
: > "$HL"; x "cd $D; ffprobe -v error -show_entries format=duration -of default=nw=1:nk=1 http://$LAN:$PORT/direct.mp4" >/dev/null 2>&1; echo "UNGUARDED ffprobe direct URL listener hits (RED expect >=1): $(hits)"
: > "$HL"; x "cd $D; ffprobe -v error $GP -show_entries format=duration -of default=nw=1:nk=1 http://$LAN:$PORT/direct.mp4" >/dev/null 2>&1; echo "GUARDED ffprobe direct URL hits (expect 0): $(hits)"
: > "$HL"; x "cd $D; ffmpeg -nostdin -protocol_whitelist file,pipe -format_whitelist $DEM -ss 0.5 -i http://$LAN:$PORT/direct.mp4 -vframes 1 -y o.jpg" >/dev/null 2>&1; echo "GUARDED ffmpeg direct URL hits (expect 0): $(hits)"
: > "$HL"; x "cd $D; ffprobe -v error $GP -show_entries format=duration -of default=nw=1:nk=1 net.m3u8" >/dev/null 2>&1; echo "GUARDED ffprobe net.m3u8 (playlist naming http seg) hits (expect 0): $(hits)"
: > "$HL"; x "cd $D; ffprobe -v error -show_entries format=duration -of default=nw=1:nk=1 net.m3u8" >/dev/null 2>&1; echo "UNGUARDED ffprobe net.m3u8 hits (informational): $(hits)"
echo "== deployed binary string presence (F2/F4/F5 code in the running image; not behaviour)"
echo "format_whitelist=$(x 'grep -c format_whitelist /app/catalogizer-api') download_exceeds=$(x 'grep -c "cover_art: download exceeds" /app/catalogizer-api') not_regular_file=$(x 'grep -c "not a regular file" /app/catalogizer-api') nostdin=$(x 'grep -c -e -nostdin /app/catalogizer-api')"
echo "ffprobe version: $(x 'ffprobe -version | head -1')"
