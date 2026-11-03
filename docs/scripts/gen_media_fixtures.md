# gen_media_fixtures.sh

`scripts/test-infra/gen_media_fixtures.sh` generates the deterministic media and document corpus used by the live validation matrix (chunk L1: playback L5, conversions L6, documents L7, subtitles L10). Everything is synthetic (ffmpeg lavfi sources, ImageMagick presence check, python): no downloads, no mocks, no host fonts.

## Usage

```
scripts/test-infra/gen_media_fixtures.sh --out DIR            # generate (DIR is emptied first)
scripts/test-infra/gen_media_fixtures.sh --out DIR --check    # generate twice into DIR/run1, DIR/run2; exit 0 identical, 1 differs
scripts/test-infra/gen_media_fixtures.sh --out DIR --verify   # re-hash DIR against DIR/manifest.json
```

`--out` must be an absolute path OUTSIDE the repository (the script refuses otherwise); with no `--out`, `$QA_WORK/media` is used. Exit codes: 0 ok, 1 failed (drift, nondeterminism, corpus over cap), 2 usage or refused, 3 tool missing.
Environment: `GMF_MAX_BYTES` (default 314572800, the 300 MB cap), `GMF_TEST_NONDET=1` (TEST ONLY: adds `-metadata creation_time=now` to one mp4 so that `--check` must go RED).

Run it under the host-safety envelope: `systemd-run --user --scope -p MemoryMax=3G -p MemorySwapMax=0 scripts/test-infra/gen_media_fixtures.sh --out "$QA_WORK/media"`. Measured: about 23 s, about 2.3 MB, 36 files.

## Corpus (layout is scanner-friendly)

| Directory | Content |
|---|---|
| `movies/` | h264/aac mp4 (`Movie.Name.2020.1080p.mp4`, 5 s 640x360), vp9/opus webm, av1/opus webm, hevc mp4, 1080p 3 s, 2160p 1 s, 60 s seek file; subtitles beside the main movie: `.en.srt`, `.en.vtt`, `.en.ass`, plus `.out-of-sync.srt` (+7 s) |
| `tv/Show Name/Season 01/` | `Show Name S01E02.mkv` (h264/aac) |
| `music/` | `Misc/Plain Tone.{mp3,flac,ogg,opus,wav,m4a}` (3 s, 440 Hz), `Misc/Silent Control.wav` (RMS negative control), `Test Artist - Test Album (2020)/01 - First Track.mp3` (ID3 artist/album/title/track/date + PNG cover) |
| `books/` | `Known Text Document.pdf` (3 pages, known text, hand-built), `Test Comic 001.cbz` (3 png pages), `Test Book.epub` (2 chapters), `Plain Notes.txt` |
| `photos/` | png, jpg, webp, animated gif, svg, a 6000x6000 png of about 120 KB |
| `negative/` | zero-byte mp4, garbage mp3, not-a-pdf, truncated mp4, truncated png |
| `manifest.json` | per file: sha256, size, class, ffprobe facts (format, duration, codec, size, rate, channels) and `expected` content (cue text and starts, PDF page text, epub text, ID3 tags, tone frequency, silent flag) |

Video frames carry the lavfi `testsrc` burned-in timestamp counter (a frame oracle that needs no font); the audio is a 440 Hz sine.

## Determinism

Every ffmpeg call uses `-threads 1`, `-bitexact`, `-fflags +bitexact`, `-map_metadata -1`, fixed encoder settings (x264 threads=1, x265 frame-threads=1 no-wpp, libaom cpu-used 8 row-mt 0, libvpx row-mt 0). Gradients are pinned by colours, geometry AND seed (the gradients filter is random if only the seed is given: measured, it changed png, jpg, webp, cbz, the cover mp3 and the truncated png between runs until the colours and the geometry were pinned too). Zip members use a fixed timestamp; the manifest has no timestamps or absolute paths.

`--check` evidence (2026-11-03): with `GMF_TEST_NONDET=1` it exits 1 and names `movies/Movie.Name.2020.1080p.mp4`, its derived `negative/truncated-tail.mp4` and `manifest.json`; without the flag it exits 0 with identical sha256 over all 36 files and an identical manifest hash.

## Honest gaps

- Not generated: EXIF-rotated jpg (no EXIF writer was verified), multi-audio-track or embedded-subtitle containers, a real-world codec corpus. The content is synthetic, so metadata providers (TMDB and similar) will not recognise the titles: matching is exercised by file naming only.
- The PDF is hand-built Helvetica text, not produced by a document renderer; `pdftotext` reads the three known lines.
- Hash equality is proven on this host and this ffmpeg build; another ffmpeg or encoder version may produce different bytes. The manifest records ffprobe facts, so consumers can fall back to facts rather than hashes across hosts.
