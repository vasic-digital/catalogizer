#!/bin/sh
# Runs INSIDE a container. Usage: smoke_media.sh gen|convert <dir>
# gen:     generates the input corpus (lavfi) into <dir>/in (run once, with the OLD image)
# convert: runs the exact argument shapes of catalog-api (services/conversion_service.go,
#          converter_hardening.go, cover_art_service.go, subtitle_sync.go, conversion_integrity.go)
#          over <dir>/in into <dir>/out and prints one TSV row of ffprobe-derived properties per output.
set -u
MODE=$1; D=$2; IN=$D/in; OUT=$D/out
DEM="mov,mp4,m4a,3gp,3g2,mj2,matroska,webm,avi,asf,flv,mpegts,mpeg,mpegvideo,ogg,wav,flac,mp3,aac,ac3,eac3,dts,ape,wv,aiff,caf,amr,au,w64,rm,h264,hevc,m4v,ivf,obu"
GUARD="-nostdin -hide_banner -nostats -loglevel error -protocol_whitelist file,pipe -format_whitelist $DEM"
PGUARD="-hide_banner -loglevel error -protocol_whitelist file,pipe -format_whitelist $DEM"
L="-f lavfi"
if [ "$MODE" = gen ]; then
  mkdir -p "$IN"; cd "$IN" || exit 1
  V="$L -i testsrc=duration=2:size=176x144:rate=15 $L -i sine=frequency=440:duration=2"
  for e in mp4 avi mkv mov flv webm m4v; do ffmpeg -hide_banner -loglevel error -y $V -shortest v.$e || echo "GENFAIL v.$e"; done
  ffmpeg -hide_banner -loglevel error -y $V -c:v wmv2 -c:a wmav2 -shortest v.wmv || echo "GENFAIL v.wmv"
  ffmpeg -hide_banner -loglevel error -y $V -c:v h263 -c:a aac -shortest v.3gp || echo "GENFAIL v.3gp"
  A="$L -i sine=frequency=330:duration=2"
  for e in mp3 wav flac aac ogg m4a opus; do ffmpeg -hide_banner -loglevel error -y $A a.$e || echo "GENFAIL a.$e"; done
  ffmpeg -hide_banner -loglevel error -y $A -c:a wmav2 a.wma || echo "GENFAIL a.wma"
  # round 2 (review F2): realistic inputs
  ffmpeg -hide_banner -loglevel error -y $L -i testsrc=d=1:s=64x64 -frames:v 1 cov.png
  ffmpeg -hide_banner -loglevel error -y $L -i testsrc=d=1:s=64x64 -frames:v 1 cov.jpg
  ffmpeg -hide_banner -loglevel error -y $L -i sine=d=2 -i cov.jpg -map 0 -map 1 -c:v copy -disposition:v attached_pic art.mp3 || echo "GENFAIL art.mp3"
  ffmpeg -hide_banner -loglevel error -y $L -i sine=d=2 -i cov.png -map 0 -map 1 -c:a flac -c:v copy -disposition:v attached_pic artpng.flac || echo "GENFAIL artpng.flac"
  printf '1\n00:00:00,000 --> 00:00:01,000\nhello\n' > s.srt
  ffmpeg -hide_banner -loglevel error -y $L -i testsrc=d=2:s=176x144:r=15 $L -i sine=d=2 -i s.srt -map 0 -map 1 -map 2 -c:v libx264 -c:a ac3 -c:s ass -shortest sub.mkv || echo "GENFAIL sub.mkv"
  ffmpeg -hide_banner -loglevel error -y $L -i testsrc=d=2:s=176x144:r=15 -c:v libx265 -pix_fmt yuv420p10le hevc10.mkv 2>/dev/null || echo "GENFAIL hevc10.mkv"
  ffmpeg -hide_banner -loglevel error -y $L -i testsrc=d=1:s=176x144:r=15 -c:v libaom-av1 -cpu-used 8 av1.mkv 2>/dev/null || echo "GENFAIL av1.mkv"
  ffmpeg -hide_banner -loglevel error -y $L -i testsrc=d=1:s=176x144:r=15 -c:v mpeg2video -c:a mp2 v.mpg || echo "GENFAIL v.mpg"
  ls -l; exit 0
fi
mkdir -p "$OUT"; cd "$IN" || exit 1
props() { # file -> TSV props
  f=$1
  [ -s "$f" ] || { echo "MISSING"; return; }
  case $f in *.jpg|*.png|*.gif|*.bmp|*.tiff|*.webp) PG="-hide_banner -loglevel error -protocol_whitelist file,pipe";; *) PG=$PGUARD;; esac
  ffprobe $PG -v error -show_entries format=format_name:stream=codec_name,codec_type,width,height,channels,sample_rate -of csv=p=0 "$f" 2>&1 | sed 's/0x[0-9a-f]*/PTR/g' | tr '\n' ';'
  d=$(ffprobe $PG -show_entries format=duration -of csv=p=0 "file:$f" 2>&1); printf '\tdur=%.1f' "$d" 2>/dev/null
}
row() { printf '%s\t%s\t%s\n' "$1" "$2" "$(props "$3")"; }
# conversion_service.go buildFFmpegVideoArgs: -i src -y -crf/-preset [settings] target
for src in v.mp4 v.avi v.mkv v.mov v.wmv v.flv v.webm v.m4v v.3gp; do
  for dst in mp4 avi mkv mov webm m4v; do
    o=$OUT/vid_${src%.*}_${src#*.}.$dst
    ffmpeg $GUARD -i "$src" -y -crf 28 -preset fast -s 160x120 -r 10 -b:v 200k "$o" >/dev/null 2>&1; rc=$?
    row "video ${src}->${dst}" "rc=$rc" "$o"
  done
done
for q in "low:-crf 28 -preset fast" "medium:-crf 23 -preset medium" "high:-crf 18 -preset slow" "lossless:-crf 0 -preset veryslow"; do
  n=${q%%:*}; a=${q#*:}; o=$OUT/vq_$n.mp4
  ffmpeg $GUARD -i v.mp4 -y $a "$o" >/dev/null 2>&1; rc=$?; row "videoq $n" "rc=$rc" "$o"
done
# buildFFmpegAudioArgs
for src in a.mp3 a.wav a.flac a.aac a.ogg a.wma a.m4a a.opus; do
  for dst in mp3 wav flac aac ogg m4a opus; do
    o=$OUT/aud_${src#*.}.$dst
    ffmpeg $GUARD -i "$src" -y -ab 192k -ar 44100 -ac 2 "$o" >/dev/null 2>&1; rc=$?
    row "audio ${src}->${dst}" "rc=$rc" "$o"
  done
done
for q in "low:-ab 96k" "medium:-ab 192k" "high:-ab 320k" "lossless:-c:a flac"; do
  n=${q%%:*}; a=${q#*:}; o=$OUT/aq_$n.${n}.out
  case $n in lossless) o=$OUT/aq_$n.flac;; *) o=$OUT/aq_$n.mp3;; esac
  ffmpeg $GUARD -i a.wav -y $a "$o" >/dev/null 2>&1; rc=$?; row "audioq $n" "rc=$rc" "$o"
done
# opus target without -ar (opus muxer rejects 44100 Hz; pre-existing behaviour, same in old and new)
for src in a.wav a.mp3 a.flac; do o=$OUT/opus_${src#*.}.opus; ffmpeg $GUARD -i "$src" -y -ab 192k "$o" >/dev/null 2>&1; rc=$?; row "audio-noar ${src}->opus" "rc=$rc" "$o"; done
# cover_art_service.go ffmpegFrameCmd + ffprobeDurationCmd
o=$OUT/frame.jpg
ffmpeg -nostdin -protocol_whitelist file,pipe -format_whitelist $DEM -ss 1.00 -i v.mp4 -vframes 1 -q:v 2 -y "$o" >/dev/null 2>&1; rc=$?; row "coverart frame" "rc=$rc" "$o"
printf 'coverart dur\t%s\n' "$(ffprobe -v error -protocol_whitelist file,pipe -format_whitelist $DEM -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 v.mp4 2>&1)"
# conversion_integrity.go
printf 'integrity dur\t%s\n' "$(ffprobe $PGUARD -show_entries format=duration -of csv=p=0 file:v.mp4 2>&1)"
# subtitle_sync.go (silencedetect on a tone-with-silence track)
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "sine=frequency=440:duration=1" -f lavfi -i "anullsrc=d=1" -filter_complex concat=n=2:v=0:a=1 sil.wav 2>/dev/null
printf 'silencedetect\t%s\n' "$(ffmpeg -nostdin -nostats -threads 1 -protocol_whitelist file -t 5.000 -i file:sil.wav -vn -af silencedetect=noise=-35dB:d=0.3 -f null - 2>&1 | grep -o 'silence_[a-z]*: [0-9.]*' | sed 's/\.[0-9]*$//' | tr '\n' ' ')"
# probe JSON checksum (stream layout, no encoder/version tags)
for f in v.mp4 v.mkv a.flac; do printf 'probejson %s\t%s\n' $f "$(ffprobe -v error -show_streams -show_format -of json $f | grep -v -E 'encoder|\"version\"|probe_score|filename|size|bit_rate' | sha256sum | cut -c1-16)"; done
# feature set the code relies on
printf 'caps\tnonexist-protocol-refused=%s\n' "$(ffmpeg -nostdin -hide_banner -loglevel error -protocol_whitelist file,pipe -i http://127.0.0.1:9/x.mp4 -f null - 2>&1 | sed 's/0x[0-9a-f]*/PTR/g' | head -c 80 | tr '\n' ' ')"
# ImageMagick (conversion_service.go buildImageMagickArgs); policy-free here, tool presence + coders
if command -v convert >/dev/null 2>&1; then
  convert -size 64x48 gradient:red-blue "$OUT/g.png" && for e in jpg gif bmp tiff webp png; do convert "$OUT/g.png" -resize 32x24 -quality 80 "$OUT/o.$e"; row "image png->$e" "rc=$?" "$OUT/o.$e"; done
fi
# ---- round 2 (review F2): realistic inputs. -vn mirrors services/conversion_service.go audioTargetsWithoutPicture (ogg, m4a)
pp() { [ -s "$1" ] || { echo MISSING; return; }; ffprobe -v error -show_entries stream=codec_name,codec_type,width,height:stream_disposition=attached_pic -of csv=p=0 "$1" 2>&1 | tr '\n' ';'; }
for s in art.mp3 artpng.flac; do for d in mp3 wav flac aac ogg m4a opus; do
  o=$OUT/${s%.*}.$d; VN=""; case $d in ogg|m4a) VN="-vn";; esac
  ffmpeg $GUARD -i $s -y $VN -ab 192k $o >/dev/null 2>&1; rc=$?; echo "cover-audio $s->$d	rc=$rc	$(pp $o)"; done; done
for s in sub.mkv hevc10.mkv av1.mkv v.mpg; do for d in mp4 mkv mov webm avi m4v; do
  o=$OUT/${s%.*}_x.$d; ffmpeg $GUARD -i $s -y -crf 28 -preset fast $o >/dev/null 2>&1; rc=$?; echo "real-video $s->$d	rc=$rc	$(pp $o)"; done
  o=$OUT/${s%.*}_f.jpg; ffmpeg -nostdin -protocol_whitelist file,pipe -format_whitelist $DEM -ss 0.50 -i $s -vframes 1 -q:v 2 -y $o >/dev/null 2>&1; rc=$?; echo "real-frame $s	rc=$rc	$(pp $o)"; done
# the PASS/FAIL verdict of these rows is computed by check_smoke_rows.sh (rc != 0 is a FAIL except the known -ar 44100 opus quirk)
