package services

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Converter hardening (follow-up F1 of the registration-approval review). ffmpeg and ImageMagick
// run on files users put under the data roots, so a crafted file is hostile input:
//
//   - ffmpeg: playlist-like demuxers (HLS, concat, DASH, SDP, ...) make it open OTHER files and
//     URLs named inside the input. Only plain media containers are accepted (-format_whitelist),
//     only the file and pipe protocols exist (-protocol_whitelist), stdin is closed (-nostdin).
//     Measured before the fix: an HLS playlist under the data root naming file:///<outside>/x.ts
//     made ffmpeg read and convert that out-of-root file.
//   - ImageMagick: an embedded policy.xml (MAGICK_CONFIGURE_PATH) denies the active/indirect coders
//     (MVG, MSL, URL, HTTP(S), FTP, EPHEMERAL, TEXT, LABEL, ...), "@file" paths and sets resource
//     limits; the input coder is forced from the file extension (a PNG-named MSL file is not read
//     as MSL); SVG sources are checked for external references first (measured before the fix: an
//     SVG made ImageMagick read an out-of-root PNG and connect to a loopback listener).
//   - both tools run under prlimit (CPU, address space, file size, open files, no core), in their
//     own process group killed on cancel (setProcessGroup, pgid > 1 guard, constitution 11.4.263),
//     under a hard wall-clock timeout even when the caller passed no deadline.

//go:embed converterpolicy/policy.xml
var imageMagickPolicy []byte

// ImageMagickPolicy returns the embedded security policy (the Dockerfiles install the same file).
func ImageMagickPolicy() []byte { return append([]byte(nil), imageMagickPolicy...) }

const (
	// converterMaxOutputEnv overrides the per-tool output size cap (RLIMIT_FSIZE, bytes).
	converterMaxOutputEnv     = "CATALOGIZER_CONVERSION_MAX_OUTPUT_BYTES"
	converterDefaultMaxOutput = 32 << 30
	converterOpenFiles        = 1024
	converterFFmpegMinAS      = 4 << 30
	converterFFmpegMaxAS      = 32 << 30
	converterMagickMinAS      = 3 << 30
	converterMagickMaxAS      = 8 << 30
	svgMaxBytes               = 16 << 20
)

// ffmpegDemuxers are the container/stream demuxers accepted for conversion input: the formats of
// GetSupportedFormats (video and audio) and their usual sibling names. Everything else
// (hls, concat, dash, sdp, rtp, rtsp, image2 patterns, lavfi, ...) is refused by -format_whitelist.
var ffmpegDemuxers = []string{
	"mov", "mp4", "m4a", "3gp", "3g2", "mj2", "matroska", "webm", "avi", "asf", "flv",
	"mpegts", "mpeg", "mpegvideo", "ogg", "wav", "flac", "mp3", "aac", "ac3", "eac3", "dts",
	"ape", "wv", "aiff", "caf", "amr", "au", "w64", "rm", "h264", "hevc", "m4v", "ivf", "obu",
}

// ffmpegGuardArgs are the global/input options placed before every ffmpeg invocation.
func ffmpegGuardArgs() []string {
	return []string{
		"-nostdin", "-hide_banner", "-nostats", "-loglevel", "error",
		"-protocol_whitelist", "file,pipe",
		"-format_whitelist", strings.Join(ffmpegDemuxers, ","),
	}
}

var (
	policyOnce sync.Once
	policyDir  string
	policyErr  error
)

// imageMagickPolicyDir writes the embedded policy into a private directory once per process and
// returns it (for MAGICK_CONFIGURE_PATH).
func imageMagickPolicyDir() (string, error) {
	policyOnce.Do(func() {
		d, err := os.MkdirTemp("", "catalogizer-magick-policy-")
		if err != nil {
			policyErr = err
			return
		}
		if err := os.WriteFile(filepath.Join(d, "policy.xml"), imageMagickPolicy, 0o444); err != nil {
			policyErr = err
			return
		}
		policyDir = d
	})
	return policyDir, policyErr
}

// isHardenedTool reports whether name goes through the hardened launcher.
func isHardenedTool(name string) bool {
	switch name {
	case "ffmpeg", "convert", "magick":
		return true
	}
	return false
}

// guardToolArgs prepends the per-tool guard options.
func guardToolArgs(name string, args []string) []string {
	if name == "ffmpeg" {
		return append(ffmpegGuardArgs(), args...)
	}
	return args
}

func converterMaxOutput() uint64 {
	if v := os.Getenv(converterMaxOutputEnv); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return converterDefaultMaxOutput
}

// converterAddressSpace sizes RLIMIT_AS to the host like the PDF parsers (MemAvailable / (2 *
// slots), clamped), with floors high enough for video encoders and ImageMagick's own memory
// policy.
func converterAddressSpace(name string) uint64 {
	min, max := uint64(converterMagickMinAS), uint64(converterMagickMaxAS)
	if name == "ffmpeg" {
		min, max = converterFFmpegMinAS, converterFFmpegMaxAS
	}
	as := min
	if avail, ok := memAvailable(); ok {
		as = avail / (2 * pdfConcurrentSlots)
		if as > max {
			as = max
		}
		if as < min {
			as = min
		}
	}
	return as
}

// converterCPUSeconds is the CPU-time backstop: the remaining wall-clock budget times the number
// of CPUs (a multi-threaded encoder burns CPU time faster than wall time), capped at one week.
func converterCPUSeconds(ctx context.Context) int {
	budget := DefaultConversionTimeout
	if d, ok := ctx.Deadline(); ok {
		budget = time.Until(d)
	}
	if budget < time.Minute {
		budget = time.Minute
	}
	secs := int(budget.Seconds()+60) * runtime.NumCPU()
	if max := 7 * 24 * 3600; secs > max {
		secs = max
	}
	return secs
}

// hardenedCommand builds the prlimit-wrapped command of a hardened tool. bin is the resolved path.
func hardenedCommand(ctx context.Context, name, bin string, args []string) (*exec.Cmd, error) {
	pl, err := lookPath("prlimit")
	if err != nil {
		return nil, fmt.Errorf("refusing to run %s without prlimit (install util-linux): an uncapped converter could exhaust host memory or CPU", name)
	}
	full := []string{
		"--as=" + strconv.FormatUint(converterAddressSpace(name), 10),
		"--cpu=" + strconv.Itoa(converterCPUSeconds(ctx)),
		"--fsize=" + strconv.FormatUint(converterMaxOutput(), 10),
		"--nofile=" + strconv.Itoa(converterOpenFiles),
		"--core=0", "--", bin,
	}
	full = append(full, guardToolArgs(name, args)...)
	cmd := exec.CommandContext(ctx, pl, full...)
	setProcessGroup(cmd)
	cmd.Env = pdfBaseEnv()
	if name != "ffmpeg" {
		dir, perr := imageMagickPolicyDir()
		if perr != nil {
			return nil, fmt.Errorf("cannot install the ImageMagick security policy: %w", perr)
		}
		cmd.Env = append(cmd.Env, "MAGICK_CONFIGURE_PATH="+dir)
	}
	return cmd, nil
}

// runHardenedTool runs a hardened tool with a bounded stderr tail kept for the error message.
func runHardenedTool(ctx context.Context, name string, args []string) error {
	bin, err := exec.LookPath(name)
	if err != nil {
		return &converterToolError{fmt.Errorf("converter tool %q is not installed on this server (not found on PATH): %w", name, err)}
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, conversionTimeoutFromEnv())
		defer cancel()
	}
	cmd, err := hardenedCommand(ctx, name, bin, args)
	if err != nil {
		return &converterToolError{err}
	}
	cmd.Stdin = nil // /dev/null
	var stderr limitedBuffer
	stderr.max = 2048
	cmd.Stdout, cmd.Stderr = io.Discard, &stderr
	if err := cmd.Run(); err != nil {
		if tail := strings.TrimSpace(stderr.String()); tail != "" {
			return &converterToolError{fmt.Errorf("%s: %w: %s", name, err, tail)}
		}
		return &converterToolError{fmt.Errorf("%s: %w", name, err)}
	}
	return nil
}

// imageMagickCoder returns the coder prefix forced on an input of the given extension, or "" when
// no forcing applies (svg is checked by checkSVGSource instead).
func imageMagickCoder(path string) (string, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "png":
		return "png:", nil
	case "jpg", "jpeg":
		return "jpeg:", nil
	case "gif":
		return "gif:", nil
	case "bmp":
		return "bmp:", nil
	case "tif", "tiff":
		return "tiff:", nil
	case "webp":
		return "webp:", nil
	case "pdf":
		return "pdf:", nil
	case "svg":
		return "", nil
	}
	return "", fmt.Errorf("source file extension %q is not a supported image input", filepath.Ext(path))
}

// errSVGExternal is returned for an SVG that references anything outside itself.
var errSVGExternal = errors.New("SVG refused: it references an external resource, a DOCTYPE/entity, a script or foreign content")

// checkSVGSource refuses an SVG that could make the renderer read another file or reach the
// network: a DOCTYPE or entity declaration, script/foreignObject/animation-with-href content, any
// href/src that is not a "#fragment" or an inline data:image URI, and url(...) /@import in
// attributes or styles that is not a "#fragment".
func checkSVGSource(path string) error {
	_, err := readCheckedSVG(path)
	return err
}

// readCheckedSVG reads the SVG once and checks the bytes it read; the caller renders THESE bytes
// (a copy in a private directory), not the path, so the file cannot change between check and use.
func readCheckedSVG(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SVG: %w", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, svgMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read SVG: %w", err)
	}
	if len(raw) > svgMaxBytes {
		return nil, fmt.Errorf("SVG refused: larger than %d bytes", svgMaxBytes)
	}
	if err := checkSVGBytes(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func checkSVGBytes(raw []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Strict = false
	dec.Entity = nil
	inStyle := 0
	sawRoot := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("SVG refused: not well-formed XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.Directive:
			return fmt.Errorf("%w (DOCTYPE/entity declaration)", errSVGExternal)
		case xml.ProcInst:
			if strings.EqualFold(t.Target, "xml-stylesheet") {
				return fmt.Errorf("%w (xml-stylesheet)", errSVGExternal)
			}
		case xml.StartElement:
			sawRoot = true
			name := strings.ToLower(t.Name.Local)
			switch name {
			case "script", "foreignobject", "iframe", "embed", "object", "audio", "video", "handler", "listener":
				return fmt.Errorf("%w (<%s>)", errSVGExternal, name)
			case "style":
				inStyle++
			}
			for _, a := range t.Attr {
				an := strings.ToLower(a.Name.Local)
				v := strings.TrimSpace(a.Value)
				switch {
				case an == "href" || an == "src" || an == "data":
					if name == "a" {
						continue // a link is never fetched while rendering
					}
					if !safeSVGRef(v) {
						return fmt.Errorf("%w (%s=%.60q)", errSVGExternal, an, v)
					}
				case strings.HasPrefix(an, "on"):
					return fmt.Errorf("%w (event handler %s)", errSVGExternal, an)
				default:
					if badCSSRef(v) {
						return fmt.Errorf("%w (%s=%.60q)", errSVGExternal, an, v)
					}
				}
			}
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, "style") && inStyle > 0 {
				inStyle--
			}
		case xml.CharData:
			if inStyle > 0 && badCSSRef(string(t)) {
				return fmt.Errorf("%w (style)", errSVGExternal)
			}
		}
	}
	if !sawRoot {
		return fmt.Errorf("SVG refused: no root element")
	}
	return nil
}

// safeSVGRef: a reference is safe when it stays inside the document or is an inline raster/svg
// data URI.
func safeSVGRef(v string) bool {
	if strings.HasPrefix(v, "#") {
		return true
	}
	l := strings.ToLower(v)
	return strings.HasPrefix(l, "data:image/png") || strings.HasPrefix(l, "data:image/jpeg") ||
		strings.HasPrefix(l, "data:image/gif") || strings.HasPrefix(l, "data:image/webp")
}

// badCSSRef reports a url(...) that is not a fragment/inline data image, or an @import.
func badCSSRef(v string) bool {
	l := strings.ToLower(v)
	if strings.Contains(l, "@import") || strings.Contains(l, "expression(") {
		return true
	}
	for rest := l; ; {
		i := strings.Index(rest, "url(")
		if i < 0 {
			return false
		}
		rest = rest[i+4:]
		arg := strings.TrimLeft(rest, " \t\r\n'\"")
		if !(strings.HasPrefix(arg, "#") || strings.HasPrefix(arg, "data:image/png") ||
			strings.HasPrefix(arg, "data:image/jpeg") || strings.HasPrefix(arg, "data:image/gif") ||
			strings.HasPrefix(arg, "data:image/webp")) {
			return true
		}
	}
}

func ctxWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
