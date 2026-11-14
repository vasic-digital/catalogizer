# Minimal ffmpeg in the catalog-api image (CAT-IMG-SLIM-1)

Status: PROVISIONAL (container-class evidence in `docs/qa/img-slim/`; no manual QA, no live media library run). Reachable from `docs/README.md` and `docs/deployment/DOCKER_SETUP.md`.

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

- An output encoder not listed above and not native (for example `-c:v libx265`, `libaom-av1`, `libtheora`) is no longer available.
  The application never selects them: video/audio encoders are chosen by the target extension
  (mp4/mkv/mov/m4v -> libx264, webm -> VP9, avi -> MPEG-4, mp3, aac, flac, wav, ogg -> vorbis, opus).
- **Cover art in audio files (found in review round 1).** For an audio file with an embedded cover picture the distro ffmpeg turned the
  picture into a second stream: for `.ogg` a `theora` video stream (a non-standard video stream inside an audio file) and for `.m4a`
  a failure (rc 234, in the distro image too). The slim ffmpeg has no libtheora, so `.ogg` failed (rc 8). Decision: do NOT add libtheora;
  `ConversionService.buildFFmpegAudioArgs` now passes `-vn` for the targets `ogg` and `m4a` (`audioTargetsWithoutPicture`), so the picture is dropped
  and the audio converts. Reason: Theora in ogg is not how players read covers (they use the vorbis `METADATA_BLOCK_PICTURE` tag), libtheora would
  add a library for a stream nobody wants, and the same `-vn` also repairs `.m4a`. mp3 and flac keep their cover (verified), wav/aac/opus drop it by themselves.
  Regression: Go tests `TestConversionService_BuildFFmpegAudioArgs` (argument shapes) and `TestHardenLive_AudioWithCoverConvertsToEveryTarget` (real ffmpeg).
- Adding a codec means adding its `-dev` package and `--enable-lib...` flag to the `ffmpeg-build` stage and
  its runtime package to the final-stage `apt-get install`; then extend `docs/qa/img-slim/required_caps.txt`.
- Runtime libraries are listed explicitly in the final stage: libx264-164 libvpx9 libmp3lame0 libopus0 libvorbis0a libvorbisenc2 libogg0
  libxvidcore4 libdav1d7 plus zlib1g libbz2-1.0 liblzma5 (the latter three are also part of the base, listing them keeps the dependency visible).
  Evidence: `ldd /usr/local/bin/ffmpeg` in `docs/qa/img-slim/ldd_ffmpeg.txt`.

## Provenance and licence

- Source: https://ffmpeg.org/releases/ffmpeg-7.1.5.tar.xz, sha256 `de668509caf9e35e3cd162473441fdb29538c6d96ed080292b3cf9e6fc5d558f` (checked in the build).
  Detached signature `ffmpeg-7.1.5.tar.xz.asc` verified by the round-1 reviewer: "Good signature" from the FFmpeg release key
  `FCF9 86EA 15E6 E293 A564  4F10 B432 2F04 D676 58D8`. No web of trust was established for that key (the fingerprint is the one published by FFmpeg); the build itself pins only the sha256.
- Licence: the build uses `--enable-gpl` with libx264 and libxvid, so the resulting `ffmpeg`/`ffprobe` binaries are GPL version 2 or later
  (not LGPL). Anyone receiving the image can obtain the corresponding source from the URL above (tarball pinned by sha256) together with the exact
  configure line, which is in `catalog-api/Dockerfile` and `catalog-api/Dockerfile.runtime` (`ffmpeg-build` stage). The Catalogizer binary only executes ffmpeg as a separate process.
  The distro ffmpeg was GPL as well (`--enable-gpl` in `docs/qa/img-slim/old_ffmpeg_config.txt`), so the licence class of the image does not change.

## Rebuild

```
podman build --network host -f catalog-api/Dockerfile -t catalogizer-api .          # from the repo root
# or, with a prebuilt binary
podman build --network host -f catalog-api/Dockerfile.runtime --build-arg BINARY_PATH=<path> -t catalogizer-api .
# bump ffmpeg: --build-arg FFMPEG_VERSION=7.1.x --build-arg FFMPEG_SHA256=<sha256 of ffmpeg-7.1.x.tar.xz>
```
Per the containerized-build rule the build runs in the build container on the designated build host.
