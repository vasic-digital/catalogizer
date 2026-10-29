package services

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Review round 7, item S2: external converters (ffmpeg, ebook-convert, ImageMagick, pandoc,
// LibreOffice) open their output by path and follow a symlink planted at the target or at a
// name they derive from it (ImageMagick "out-0.bmp" for multi-frame input, "%d" patterns).
// Each converter now writes into a fresh private directory and only regular files are moved
// into place.
//
// TEST CLASS: unit / gate-logic (§11.4.27, §11.4.289). The converters below are FAKE shell
// scripts put first on PATH; they prove that the service's own path handling refuses or
// isolates links, not that ffmpeg/ImageMagick/pandoc behave this way. The live ImageMagick
// check is TestRV7_RealImageMagickMultiFrameLink (skipped honestly when convert is absent).

// rv7FakeTool installs a fake converter. Modes (FAKE_MODE): single = write OUT to the output
// path; multi = write <base>-0.<ext> and <base>-1.<ext> (ImageMagick multi-frame naming);
// pct = substitute %d with 0 and 1 (frame patterns); link = create a symlink at the output
// path; fail = write a partial file and exit 1; none = exit 0 without writing. The output path is the last argument, the
// second for ebook-convert, the argument after -o for pandoc, and --outdir/<src>.html for
// libreoffice.
func rv7FakeTool(t *testing.T, name, mode string) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
set -u
for a in "$@"; do out="$a"; done
case "` + name + `" in
  ebook-convert) out="$2" ;;
  pandoc) prev=""; for a in "$@"; do [ "$prev" = "-o" ] && out="$a"; prev="$a"; done ;;
  libreoffice) prev=""; for a in "$@"; do [ "$prev" = "--outdir" ] && dir="$a"; prev="$a"; done
               b=$(basename "$out"); out="$dir/${b%.*}.html" ;;
esac
[ -n "${FAKE_MARK:-}" ] && : > "$FAKE_MARK"
case "$FAKE_MODE" in
  single) printf OUT > "$out" ;;
  multi)  base="${out%.*}"; ext="${out##*.}"; printf F0 > "$base-0.$ext"; printf F1 > "$base-1.$ext" ;;
  pct)    printf F0 > "$(printf "$out" 0)"; printf F1 > "$(printf "$out" 1)" ;;
  link)   ln -s /etc/hostname "$out" ;;
  fail)   printf PARTIAL > "$out"; exit 1 ;;
  none)   : ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_MODE", mode)
}

// rv7Victim plants a file outside the conversion root and returns its path.
func rv7Victim(t *testing.T, outside string) string {
	t.Helper()
	v := filepath.Join(outside, "victim")
	require.NoError(t, os.WriteFile(v, []byte("VICTIM"), 0o644))
	return v
}

func rv7AssertVictim(t *testing.T, v, msg string) {
	t.Helper()
	b, err := os.ReadFile(v)
	require.NoError(t, err)
	assert.Equal(t, "VICTIM", string(b), msg)
}

func rv7NoPrivateDirLeft(t *testing.T, dir string) {
	t.Helper()
	ents, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range ents {
		assert.False(t, strings.HasPrefix(e.Name(), ".catalogizer-conv-"), "private converter directory left behind: %s", e.Name())
	}
}

type rv7Converter struct {
	tool string
	run  func(job *models.ConversionJob) error
}

func rv7Converters() map[string]rv7Converter {
	s := &ConversionService{}
	return map[string]rv7Converter{
		"ffmpeg video":      {"ffmpeg", s.convertVideo},
		"ffmpeg audio":      {"ffmpeg", s.convertAudio},
		"ebook-convert":     {"ebook-convert", s.convertEbook},
		"imagemagick image": {"convert", s.convertImage},
		"imagemagick pdf":   {"convert", func(j *models.ConversionJob) error { return s.convertPDFWithImageMagick(j, "bmp") }},
	}
}

func TestRV7_ConvertersRefuseSymlinkAtTarget(t *testing.T) {
	for name, c := range rv7Converters() {
		t.Run(name, func(t *testing.T) {
			root, outside, src := rv6ConvFixture(t)
			rv7FakeTool(t, c.tool, "single")
			v := rv7Victim(t, outside)
			tgt := filepath.Join(root, "out.bmp")
			require.NoError(t, os.Symlink(v, tgt))
			assert.Error(t, c.run(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
			rv7AssertVictim(t, v, name+" wrote through a symlink planted at the target")
			rv7NoPrivateDirLeft(t, root)
		})
	}
}

func TestRV7_ConvertersRefuseSymlinkAtDerivedName(t *testing.T) {
	for _, tc := range []struct{ name, mode, target, planted string }{
		{"multi-frame name-N.ext", "multi", "out.bmp", "out-0.bmp"},
		{"percent-d pattern", "pct", "img-%d.png", "img-1.png"},
	} {
		for name, c := range rv7Converters() {
			if c.tool != "convert" && c.tool != "ffmpeg" {
				continue
			}
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				root, outside, src := rv6ConvFixture(t)
				rv7FakeTool(t, c.tool, tc.mode)
				v := rv7Victim(t, outside)
				require.NoError(t, os.Symlink(v, filepath.Join(root, tc.planted)))
				assert.Error(t, c.run(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(root, tc.target)}))
				rv7AssertVictim(t, v, name+" wrote a derived output through a planted symlink")
				rv7NoPrivateDirLeft(t, root)
			})
		}
	}
}

// golden-FALSE: legitimate single and multi-frame outputs land as regular files in place.
func TestRV7_ConvertersPublishRegularOutputs(t *testing.T) {
	for name, c := range rv7Converters() {
		t.Run(name, func(t *testing.T) {
			root, _, src := rv6ConvFixture(t)
			rv7FakeTool(t, c.tool, "single")
			tgt := filepath.Join(root, "out.bmp")
			require.NoError(t, c.run(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
			b, err := os.ReadFile(tgt)
			require.NoError(t, err)
			assert.Equal(t, "OUT", string(b))
			fi, err := os.Lstat(tgt)
			require.NoError(t, err)
			assert.True(t, fi.Mode().IsRegular())
			rv7NoPrivateDirLeft(t, root)
		})
	}
	t.Run("multi-frame outputs", func(t *testing.T) {
		root, _, src := rv6ConvFixture(t)
		rv7FakeTool(t, "convert", "multi")
		require.NoError(t, (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(root, "out.bmp")}))
		for _, n := range []string{"out-0.bmp", "out-1.bmp"} {
			_, err := os.Lstat(filepath.Join(root, n))
			assert.NoError(t, err, n)
		}
		rv7NoPrivateDirLeft(t, root)
	})
	t.Run("existing regular target is replaced", func(t *testing.T) {
		root, _, src := rv6ConvFixture(t)
		rv7FakeTool(t, "ffmpeg", "single")
		tgt := filepath.Join(root, "out.mp4")
		require.NoError(t, os.WriteFile(tgt, []byte("OLD"), 0o644))
		require.NoError(t, (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
		b, _ := os.ReadFile(tgt)
		assert.Equal(t, "OUT", string(b))
	})
}

func TestRV7_ConverterOutputThatIsNotARegularFileIsRefused(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	rv7FakeTool(t, "convert", "link")
	tgt := filepath.Join(root, "out.bmp")
	assert.Error(t, (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
	_, err := os.Lstat(tgt)
	assert.True(t, os.IsNotExist(err), "a symlink the tool created was published at the target")
	rv7NoPrivateDirLeft(t, root)
}

func TestRV7_FailedConverterPublishesNothing(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	rv7FakeTool(t, "ffmpeg", "fail")
	tgt := filepath.Join(root, "out.mp4")
	assert.Error(t, (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
	_, err := os.Lstat(tgt)
	assert.True(t, os.IsNotExist(err), "a failed converter's partial output was published")
	rv7NoPrivateDirLeft(t, root)
}

// pandoc and LibreOffice (the HTML path) go through the same private directory.
func TestRV7_HTMLConvertersUsePrivateDir(t *testing.T) {
	t.Run("pandoc output published", func(t *testing.T) {
		root, _, src := rv6ConvFixture(t)
		rv7FakeTool(t, "pandoc", "single")
		tgt := filepath.Join(root, "doc.html")
		require.NoError(t, (&ConversionService{}).convertPDFToHTML(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
		b, err := os.ReadFile(tgt)
		require.NoError(t, err)
		assert.Equal(t, "OUT", string(b))
		rv7NoPrivateDirLeft(t, root)
	})
	t.Run("libreoffice output published, planted sibling untouched", func(t *testing.T) {
		root, outside, src := rv6ConvFixture(t)
		rv7FakeTool(t, "pandoc", "fail")
		bin := t.TempDir()
		lo := `#!/bin/sh
prev=""; for a in "$@"; do [ "$prev" = "--outdir" ] && dir="$a"; prev="$a"; out="$a"; done
b=$(basename "$out"); printf LO > "$dir/${b%.*}.html"
`
		require.NoError(t, os.WriteFile(filepath.Join(bin, "libreoffice"), []byte(lo), 0o755))
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		v := rv7Victim(t, outside)
		// LibreOffice names its output after the source (in.html); a link planted there in the
		// target directory must neither be written through nor block the conversion.
		require.NoError(t, os.Symlink(v, filepath.Join(root, "in.html")))
		tgt := filepath.Join(root, "doc.html")
		require.NoError(t, (&ConversionService{}).convertPDFToHTML(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
		b, err := os.ReadFile(tgt)
		require.NoError(t, err)
		assert.Equal(t, "LO", string(b))
		rv7AssertVictim(t, v, "LibreOffice output was written through the planted sibling link")
		rv7NoPrivateDirLeft(t, root)
	})
}

// T1: a FIFO or a directory at the target is refused before any tool runs (not only symlinks).
func TestRV7_NonRegularTargetLeavesAreRefused(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	rv7FakeTool(t, "convert", "single")
	mark := filepath.Join(t.TempDir(), "tool-ran")
	t.Setenv("FAKE_MARK", mark)
	fifo := filepath.Join(root, "fifo.bmp")
	require.NoError(t, syscall.Mkfifo(fifo, 0o644))
	dir := filepath.Join(root, "dir.bmp")
	require.NoError(t, os.Mkdir(dir, 0o755))
	for _, tgt := range []string{fifo, dir} {
		err := (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
		require.Error(t, err, tgt)
		assert.Contains(t, err.Error(), "not a regular file", tgt)
	}
	_, serr := os.Lstat(mark)
	assert.True(t, os.IsNotExist(serr), "the converter ran although the target leaf is not a regular file")
	for _, p := range []string{fifo, dir} {
		assert.Error(t, refuseNonRegularLeaf(p), p)
	}
	rv7NoPrivateDirLeft(t, root)
}

// T1: moveLibreOfficeOutput reports an Lstat error other than "does not exist".
func TestRV7_MoveLibreOfficeOutputLstatError(t *testing.T) {
	root, _, _ := rv6ConvFixture(t)
	// a path whose parent component is a regular file: Lstat fails with ENOTDIR
	f := filepath.Join(root, "plain")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))
	err := moveLibreOfficeOutput(filepath.Join(f, "x.html"), filepath.Join(root, "t.html"))
	assert.Error(t, err)
	_, serr := os.Lstat(filepath.Join(root, "t.html"))
	assert.True(t, os.IsNotExist(serr))
}

// LIVE check with the real ImageMagick (reviewer's reproduction): multi-frame input converted to
// BMP writes out-0.bmp / out-1.bmp; a link planted at out-0.bmp must not be followed. Skipped,
// honestly, when no convert binary is on PATH (the Go test container has none; the evidence run
// uses the test binary on the host).
func TestRV7_RealImageMagickMultiFrameLink(t *testing.T) {
	if _, err := exec.LookPath("convert"); err != nil {
		t.Skip("SKIP-with-reason: hardware_not_present - no ImageMagick convert on PATH")
	}
	root, outside, _ := rv6ConvFixture(t)
	src := filepath.Join(root, "two.gif")
	g := &gif.GIF{}
	for _, c := range []color.Color{color.White, color.Black} {
		im := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.White, color.Black})
		for i := range im.Pix {
			im.Pix[i] = uint8(im.Palette.Index(c))
		}
		g.Image = append(g.Image, im)
		g.Delay = append(g.Delay, 1)
	}
	fh, err := os.Create(src)
	require.NoError(t, err)
	require.NoError(t, gif.EncodeAll(fh, g))
	require.NoError(t, fh.Close())

	v := rv7Victim(t, outside)
	require.NoError(t, os.Symlink(v, filepath.Join(root, "out-0.bmp")))
	err = (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(root, "out.bmp")})
	t.Logf("LIVE imagemagick: err=%v", err)
	assert.Error(t, err)
	rv7AssertVictim(t, v, "real ImageMagick overwrote the victim through out-0.bmp")

	// golden-FALSE: without the planted link, both frames are published as regular files
	require.NoError(t, os.Remove(filepath.Join(root, "out-0.bmp")))
	require.NoError(t, (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(root, "out.bmp")}))
	for _, n := range []string{"out-0.bmp", "out-1.bmp"} {
		fi, err := os.Lstat(filepath.Join(root, n))
		require.NoError(t, err, n)
		assert.True(t, fi.Mode().IsRegular() && fi.Size() > 0, n)
	}
	rv7NoPrivateDirLeft(t, root)
}

// A target outside the conversion write roots is refused before the private directory is created
// and before any tool runs (otherwise the private directory, and the tool's output, would land
// outside the roots for the duration of the run).
func TestRV7_ConverterTargetOutsideRootsRefusedBeforeToolRuns(t *testing.T) {
	_, outside, src := rv6ConvFixture(t)
	rv7FakeTool(t, "ffmpeg", "single")
	mark := filepath.Join(t.TempDir(), "tool-ran")
	t.Setenv("FAKE_MARK", mark)
	err := (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(outside, "out.mp4")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target path")
	_, serr := os.Lstat(mark)
	assert.True(t, os.IsNotExist(serr), "the converter ran for a target outside the roots")
	ents, rerr := os.ReadDir(outside)
	require.NoError(t, rerr)
	assert.Empty(t, ents, "something was created outside the roots")
}

// A refusal of pandoc's output (here: pandoc produced a symlink) is returned; it must not fall
// through to LibreOffice as if pandoc were merely unavailable.
func TestRV7_HTMLRefusalDoesNotFallBack(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	rv7FakeTool(t, "pandoc", "link")
	bin := t.TempDir()
	lo := `#!/bin/sh
prev=""; for a in "$@"; do [ "$prev" = "--outdir" ] && dir="$a"; prev="$a"; out="$a"; done
b=$(basename "$out"); printf LO > "$dir/${b%.*}.html"
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "libreoffice"), []byte(lo), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	tgt := filepath.Join(root, "doc.html")
	err := (&ConversionService{}).convertPDFToHTML(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")
	_, serr := os.Lstat(tgt)
	assert.True(t, os.IsNotExist(serr), "a refused pandoc output was replaced by the LibreOffice fallback")
	rv7NoPrivateDirLeft(t, root)
}

// A converter that exits 0 without writing anything is an error, not a silent success.
func TestRV7_ConverterWithNoOutputIsAnError(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	rv7FakeTool(t, "ffmpeg", "none")
	tgt := filepath.Join(root, "out.mp4")
	err := (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "produced no output")
	rv7NoPrivateDirLeft(t, root)
}

// A link planted at a derived name that points INSIDE the roots is refused too: the planted link
// is neither written through nor silently replaced (publish refuses every non-regular leaf).
func TestRV7_InRootLinkAtDerivedNameIsRefusedNotReplaced(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	rv7FakeTool(t, "convert", "multi")
	keep := filepath.Join(root, "keep")
	require.NoError(t, os.WriteFile(keep, []byte("KEEP"), 0o644))
	planted := filepath.Join(root, "out-0.bmp")
	require.NoError(t, os.Symlink(keep, planted))
	assert.Error(t, (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(root, "out.bmp")}))
	fi, err := os.Lstat(planted)
	require.NoError(t, err)
	assert.True(t, fi.Mode()&os.ModeSymlink != 0, "the planted in-root link was replaced")
	b, _ := os.ReadFile(keep)
	assert.Equal(t, "KEEP", string(b))
	_, err = os.Lstat(filepath.Join(root, "out-1.bmp"))
	assert.True(t, os.IsNotExist(err), "a frame was published although another was refused")
	rv7NoPrivateDirLeft(t, root)
}
