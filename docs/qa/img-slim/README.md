# CAT-IMG-SLIM-1 evidence (PROVISIONAL, container-class)

Goal: slim the catalog-api image (operator decision 2026-11-13) without removing any runtime capability the code uses.

Result (`podman image inspect ... --format '{{.Size}}'`, rootless podman, this host):

| image | size | note |
|---|---|---|
| old `localhost/catqa3-api:main-70d10721` (distro ffmpeg + imagemagick) | 746,431,164 B (746 MB) | the 761 MB in the follow-up doc was an earlier build; which commit is UNCONFIRMED |
| new `localhost/catimgslim-api:slim` (`Dockerfile.runtime`, same Go binary, sha256 59c6fe1d...) | 306,658,399 B (307 MB) | -439.8 MB, -59 % |

Per layer (`old_history.txt` vs `new_history.txt`): old apt layer 492 MB (ffmpeg and its dependencies about 390 MB),
`chown -R /app` layer 86.8 MB (a second copy of the binary), binary 86.7 MB, base 81.3 MB. New: apt layer 89.3 MB
(imagemagick, poppler, util-linux, wget, codec runtime libraries), ffmpeg + ffprobe 49.3 MB, binary 86.7 MB (one copy, `COPY --chown`), base 81.3 MB.
Cause analysis: `old_pkg_sizes.txt`, `old_big_libs.txt` (libLLVM 130 MB, Mesa gallium 42 MB, libz3 28 MB, flite voices, x265, svt-av1, codec2 ...);
`noffmpeg_baseline.txt`: the same package set without ffmpeg is 188 MB in total.

Proof files:
- `smoke_media.sh`: the exact argument shapes of the code (conversion_service.go video/audio args, converter_hardening.go guard args,
  cover_art_service.go frame grab and duration probe, conversion_integrity.go, subtitle_sync.go silencedetect, ImageMagick resize) over a lavfi-generated corpus
  (9 video and 8 audio input formats, 6 video and 7 audio targets, 4 qualities each, ImageMagick png to jpg/gif/bmp/tiff/webp/png).
- `old_convert_run{1,2}.tsv` vs `new_convert_run{1,2}.tsv`: ffprobe-derived properties per output (codec, container, size, channels, rate, duration) and
  probe-JSON checksums. `diff` is EMPTY for both runs (135 rows each, 120 rc=0). The 8 `->opus` rows with `-ar 44100` fail with rc=234 identically in old and new
  (opus muxer rejects 44100 Hz; pre-existing); the `audio-noar` rows show opus works without `-ar`. Both sides are deterministic across runs.
- `caps_old.txt`, `caps_new.txt`, `caps_diff.txt`: decoder/encoder/demuxer/muxer/filter lists. Missing in the new image: hardware paths (cuvid, qsv, vaapi, vulkan, v4l2m2m, nvenc) and
  external-library codecs/filters the code never selects (x265, aom, svtav1, rav1e, theora, webp, jxl, speex, gsm, codec2, libplacebo, ass, rubberband ...).
  The first build (`*_v1_no_zlib.log`) lost zlib through `--disable-autodetect` (png/apng decode); the smoke diff caught it (png probe 0x0) and
  `--enable-zlib --enable-bzlib --enable-lzma --enable-iconv` fixed it.
- `live_container_smoke.txt`: new image started rootless in a throwaway pod with a throwaway postgres:16-alpine and throwaway volumes: `/health` 200,
  `/health/deep` database healthy and converters `ffmpeg:true convert:true pdftotext:true`, user uid 1001 appuser, HEALTHCHECK command rc=0
  (`convert_pdf`/`convert_svg` false exactly as in the old image).
- `gotest_{old,new}_{services,isvc}.test.txt`: the repository's own Go tests (test binaries compiled in a golang:1.26 container, executed inside each image):
  `services` (conversion, hardening, ffmpeg): PASS 81 FAIL 2 SKIP 1 in BOTH images; `internal/services` (cover art, subtitle): PASS 192 FAIL 0 in BOTH; per-test verdict lists identical.
  The 2 failures (TestHardenLive_OrdinaryConversionsStillWork, TestRV7_ConvertersPublishRegularOutputs) are an SVG case (`convert: no images defined ok_svg.png`):
  ImageMagick has no SVG renderer in either image (known open item F5), unrelated to this change.
- `build_*.log`, `main_dockerfile_final_stage_ownership.txt`: build logs. `build_full_dockerfile_attempt1_ARG_error.log` is a failed first full build that exposed an
  ARG-before-FROM mistake (fixed: `ARG GO_IMAGE` and the ffmpeg ARGs stay before the first FROM); afterwards the main `Dockerfile` final stage was verified with a stub builder stage (ownership 1001:1001, ffmpeg present).

Commands (clone root, rootless podman, `nice -n 10`, `systemd-run --user --scope -p MemoryMax=6G`):
```
podman build --network host -f catalog-api/Dockerfile.runtime --build-arg BINARY_PATH=.tmp_bin/catalogizer-api -t localhost/catimgslim-api:slim .
podman run --rm --user 0 --entrypoint sh -v $W:/w <image> /w/smoke_media.sh gen|convert /w/data
```

UNCONFIRMED / not exercised: a full `Dockerfile` build including the Go stage (the clone has no submodules and the disk is nearly full; the Go stage lines are unchanged);
the conversion worker end to end through the HTTP API with a real library; ENCODING to wmv/flv/3gp/wma (not offered as targets, only decoded); byte equality of encodes
(compared through ffprobe properties, which matched; encoder libraries are the same Debian versions as before); the ffmpeg tarball signature (only the sha256 is pinned,
the key server was unreachable); builds on the designated remote build host (this build ran on the local host with rootless podman).
