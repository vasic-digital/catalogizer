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

// ImageMagickPolicy returns the embedded security policy (it is embedded in the binary and applied through MAGICK_CONFIGURE_PATH at every run; the Dockerfiles install nothing).
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

// ffprobeGuardArgs: the same protocol/demuxer confinement as ffmpeg (ffprobe has no -nostdin/-nostats).
func ffprobeGuardArgs() []string {
	return []string{
		"-hide_banner", "-loglevel", "error",
		"-protocol_whitelist", "file,pipe",
		"-format_whitelist", strings.Join(ffmpegDemuxers, ","),
	}
}

var (
	policyMu  sync.Mutex
	policyDir string
)

// imageMagickPolicyDir returns a private directory holding the embedded policy (for
// MAGICK_CONFIGURE_PATH) and verifies it on EVERY call: the file must exist with exactly the
// embedded bytes, otherwise it is rewritten (in a fresh directory when the old one is unusable).
// An unrecoverable failure returns an error and the job is refused: ImageMagick without the
// policy would read MVG/MSL/URL coders and "@file" paths (fail-open), so it never runs.
func imageMagickPolicyDir() (string, error) {
	policyMu.Lock()
	defer policyMu.Unlock()
	if policyDir != "" {
		pf := filepath.Join(policyDir, "policy.xml")
		if cur, err := os.ReadFile(pf); err == nil && bytes.Equal(cur, imageMagickPolicy) {
			return policyDir, nil
		}
		_ = os.Chmod(pf, 0o600)
		_ = os.Remove(pf)
		if err := os.WriteFile(pf, imageMagickPolicy, 0o444); err == nil {
			if cur, rerr := os.ReadFile(pf); rerr == nil && bytes.Equal(cur, imageMagickPolicy) {
				return policyDir, nil
			}
		}
	}
	d, err := os.MkdirTemp("", "catalogizer-magick-policy-")
	if err != nil {
		return "", err
	}
	pf := filepath.Join(d, "policy.xml")
	if err := os.WriteFile(pf, imageMagickPolicy, 0o444); err != nil {
		return "", err
	}
	if cur, rerr := os.ReadFile(pf); rerr != nil || !bytes.Equal(cur, imageMagickPolicy) {
		return "", errors.New("the ImageMagick security policy could not be verified after writing")
	}
	policyDir = d
	return d, nil
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
	if name == "ffprobe" {
		return append(ffprobeGuardArgs(), args...)
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
	if name == "ffmpeg" || name == "ffprobe" {
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
	if name != "ffmpeg" && name != "ffprobe" {
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
// the svg coder is forced too; the file itself is the re-serialised copy from readCheckedSVG.
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
		return "svg:", nil
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
	return sanitizeSVG(raw)
}

const (
	svgNS   = "http://www.w3.org/2000/svg"
	xlinkNS = "http://www.w3.org/1999/xlink"
	xmlNS   = "http://www.w3.org/XML/1998/namespace"
)

// svgName is the only shape of element/attribute local name that is re-emitted.
var svgNameOK = func(n string) bool {
	if n == "" {
		return false
	}
	for i, r := range n {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case i > 0 && (r >= '0' && r <= '9' || r == '-' || r == '.'):
		default:
			return false
		}
	}
	return true
}

// checkSVGBytes validates raw (see sanitizeSVG).
func checkSVGBytes(raw []byte) error {
	_, err := sanitizeSVG(raw)
	return err
}

// sanitizeSVG does not trust Go's XML parse as a model of the renderer (libxml2/Inkscape resolve
// namespace prefixes, duplicate declarations and odd names differently). It RE-SERIALISES the
// document from the parsed tree: only elements in the SVG namespace, only unprefixed / SVG /
// xlink:* / xml:* attributes under canonical fixed prefixes, one xmlns declaration pair on the root;
// comments, processing instructions, DOCTYPE, foreign elements (with their subtree), foreign or
// ambiguous attributes and every original namespace declaration are dropped. The renderer is given
// THESE bytes, so it can only see what is allowed here. A prefixed attribute whose prefix Go maps
// to the empty namespace is re-emitted as its bare local name, which no delegate treats as a path.
func sanitizeSVG(raw []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Strict = false
	dec.Entity = nil
	var out bytes.Buffer
	var stack []string // emitted element names; "" marks a dropped element
	inStyle := 0
	sawRoot, rootDone := false, false
	skip := 0 // depth inside a dropped subtree
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("SVG refused: not well-formed XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("%w (DOCTYPE/entity declaration)", errSVGExternal)
		case xml.ProcInst:
			if strings.EqualFold(t.Target, "xml-stylesheet") {
				return nil, fmt.Errorf("%w (xml-stylesheet)", errSVGExternal)
			}
		case xml.Comment:
		case xml.StartElement:
			if rootDone {
				return nil, errors.New("SVG refused: content after the root element")
			}
			if skip > 0 {
				skip++
				continue
			}
			if !sawRoot && (t.Name.Space != svgNS || t.Name.Local != "svg") {
				return nil, errors.New("SVG refused: the root element must be <svg> in the SVG namespace")
			}
			name := strings.ToLower(t.Name.Local)
			switch name {
			case "script", "foreignobject", "iframe", "embed", "object", "audio", "video", "handler", "listener":
				return nil, fmt.Errorf("%w (<%s>)", errSVGExternal, name)
			}
			if t.Name.Space != svgNS || !svgNameOK(t.Name.Local) {
				skip = 1 // foreign element: dropped with its subtree
				continue
			}
			out.WriteString("<" + t.Name.Local)
			if !sawRoot {
				out.WriteString(` xmlns="` + svgNS + `" xmlns:xlink="` + xlinkNS + `"`)
			}
			sawRoot = true
			seen := map[string]bool{}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue // original declarations are dropped
				}
				var emit string
				switch a.Name.Space {
				case "", svgNS:
					emit = a.Name.Local
				case xlinkNS:
					switch a.Name.Local {
					case "href", "title", "role", "arcrole", "show", "actuate", "type":
					default:
						return nil, fmt.Errorf("%w (attribute xlink:%s)", errSVGExternal, a.Name.Local)
					}
					emit = "xlink:" + a.Name.Local
				case xmlNS:
					emit = "xml:" + a.Name.Local
				default:
					// foreign or ambiguous namespace (sodipodi, inkscape, a literal "xml" bound
					// through xmlns:x="xml", ...): refuse, never guess how a delegate reads it
					return nil, fmt.Errorf("%w (attribute %s:%s in a foreign namespace)", errSVGExternal, a.Name.Space, a.Name.Local)
				}
				if !svgNameOK(a.Name.Local) {
					return nil, fmt.Errorf("SVG refused: unusual attribute name %.40q", a.Name.Local)
				}
				if seen[emit] {
					return nil, fmt.Errorf("SVG refused: duplicate attribute %s", emit)
				}
				seen[emit] = true
				an := strings.ToLower(a.Name.Local)
				v := strings.TrimSpace(a.Value)
				switch {
				case an == "href" || an == "src" || an == "data":
					if name == "a" {
						continue // a link is never fetched while rendering: dropped
					}
					if !safeSVGRef(v) {
						return nil, fmt.Errorf("%w (%s=%.60q)", errSVGExternal, an, v)
					}
				case strings.HasPrefix(an, "on"):
					return nil, fmt.Errorf("%w (event handler %s)", errSVGExternal, an)
				default:
					// no SVG attribute value is a file path or URL (hrefs are handled above): an
					// absolute path or scheme in any other attribute is a delegate-specific path
					// attribute (absref and friends) and is refused
					if badCSSRef(v) || strings.HasPrefix(v, "/") || strings.HasPrefix(v, "\\") ||
						strings.Contains(v, "://") || strings.Contains(strings.ToLower(v), "file:") {
						return nil, fmt.Errorf("%w (%s=%.60q)", errSVGExternal, an, v)
					}
				}
				out.WriteString(" " + emit + `="`)
				_ = xml.EscapeText(&out, []byte(a.Value))
				out.WriteString(`"`)
			}
			out.WriteString(">")
			stack = append(stack, t.Name.Local)
			if name == "style" {
				inStyle++
			}
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			if len(stack) == 0 {
				continue
			}
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			out.WriteString("</" + n + ">")
			if strings.EqualFold(n, "style") && inStyle > 0 {
				inStyle--
			}
			if len(stack) == 0 {
				rootDone = true
			}
		case xml.CharData:
			if !sawRoot {
				if len(bytes.TrimSpace(t)) != 0 {
					return nil, errors.New("SVG refused: text before the root element (content sniffing)")
				}
				continue
			}
			if skip > 0 || rootDone {
				if rootDone && len(bytes.TrimSpace(t)) != 0 {
					return nil, errors.New("SVG refused: text after the root element")
				}
				continue
			}
			if inStyle > 0 && badCSSRef(string(t)) {
				return nil, fmt.Errorf("%w (style)", errSVGExternal)
			}
			_ = xml.EscapeText(&out, t)
		}
	}
	if !sawRoot {
		return nil, errors.New("SVG refused: no root element")
	}
	if len(stack) != 0 {
		return nil, errors.New("SVG refused: unclosed element")
	}
	return out.Bytes(), nil
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
