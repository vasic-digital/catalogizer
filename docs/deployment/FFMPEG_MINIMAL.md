# Minimal ffmpeg in the catalog-api image (CAT-IMG-SLIM-1)

Status: PROVISIONAL (container-class evidence in `docs/qa/img-slim/`; no manual QA, no live media library run).

## Why

The distro `ffmpeg` package links `libavfilter` against `libplacebo`/Vulkan, which pulls Mesa gallium,
`libLLVM` (130 MB), `libz3`, the flite speech voices, x265, SVT-AV1, codec2 and about 150 other libraries.
That was about 400 MB of the 746-761 MB API image. Catalogizer never calls those features.

## What the image now contains

`catalog-api/Dockerfile` and `catalog-api/Dockerfile.runtime` build `ffmpeg` and `ffprobe` (version and
sha256 pinned in the `FFMPEG_VERSION` / `FFMPEG_SHA256` build args) in a throw-away `ffmpeg-build` stage and
copy only the two stripped binaries to `/usr/local/bin`. The libav* libraries are linked statically.

Included:
- every native decoder, demuxer, muxer, encoder, parser, bitstream filter and filter of ffmpeg 7.1
  (so every input format the code lists, `silencedetect`, `scale`, `-vframes` frame grabs, `ffprobe` probing);
- external libraries: `libx264` (H.264 for mp4/mkv/mov/m4v), `libvpx` (VP9 for webm), `libmp3lame` (mp3),
  `libopus` (opus), `libvorbis` (ogg), `libxvid` (MPEG-4 part 2 for avi), `libdav1d` (AV1 decoding),
  linked dynamically to the matching Debian runtime packages installed in the final stage.

Excluded (`--disable-autodetect`): Vulkan/libplacebo, VAAPI/QSV/other hardware acceleration, X11/SDL/ALSA/Pulse
devices, network protocol libraries (the code only allows `file,pipe` anyway), x265, SVT-AV1, aom, rav1e,
Theora, Speex, WebP, OpenJPEG, JPEG XL, subtitle rendering (libass/fontconfig), zimg/vidstab/rubberband and
other filters that need an external library, ffplay and docs.

Exact call shapes that were exercised are in `catalog-api/services/conversion_service.go`,
`converter_hardening.go`, `internal/services/cover_art_service.go`, `subtitle_sync.go`,
`services/conversion_integrity.go`; the check is `docs/qa/img-slim/smoke_media.sh`.

## Consequences

- An output encoder not listed above and not native (for example `-c:v libx265`, `libaom-av1`) is no longer available.
  The application never selects them: video/audio encoders are chosen by the target extension
  (mp4/mkv/mov/m4v -> libx264, webm -> VP9, avi -> MPEG-4, mp3, aac, flac, wav, ogg -> vorbis, opus).
- Adding a codec means adding its `-dev` package and `--enable-lib...` flag to the `ffmpeg-build` stage and
  its runtime package to the final-stage `apt-get install`.
- The source tarball is pinned by sha256 only; its detached signature (key FCF986EA15E6E293A5644F10B4322F04D67658D8)
  could not be verified when this was written (key server unreachable): UNCONFIRMED authenticity beyond the hash.
- The ffmpeg build adds about 7 minutes (measured, `make -j8`, nice 10; cached afterwards) (`make -j` is capped at 8 for host safety).

## Rebuild

```
podman build --network host -f catalog-api/Dockerfile -t catalogizer-api .          # from the repo root
# or, with a prebuilt binary
podman build --network host -f catalog-api/Dockerfile.runtime --build-arg BINARY_PATH=<path> -t catalogizer-api .
# bump ffmpeg: --build-arg FFMPEG_VERSION=7.1.x --build-arg FFMPEG_SHA256=<sha256 of ffmpeg-7.1.x.tar.xz>
```
Per the containerized-build rule the build runs in the build container on the designated build host.
