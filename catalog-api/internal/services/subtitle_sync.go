package services

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Sync statuses reported by VerifySynchronization.
const (
	// SyncStatusMeasured: an offset was measured against the real audio of the media file.
	SyncStatusMeasured = "measured"
	// SyncStatusUndetermined: no honest measurement was possible (reason says why); offset 0, confidence 0.
	SyncStatusUndetermined = "undetermined"
	// SyncStatusInvalid: the subtitle is structurally unusable for this media (no cues, cues beyond the media end).
	SyncStatusInvalid = "invalid"
)

const (
	syncBinSeconds    = 0.1  // resolution of the activity signals
	syncMaxShiftSecs  = 120. // search window for the delay: +-120 s
	syncMaxAnalysis   = 1800 // only the first 30 minutes of the media are analysed
	syncValidOffsetMs = 500. // |offset| within this is "in sync"
	syncDurationSlack = 2.0  // seconds a last cue may run past the media end
	syncMinPeak       = 0.30 // minimum normalised correlation of the best alignment
	syncMinMargin     = 0.10 // minimum lead over the best alignment >2 s away (rejects ambiguous patterns)
)

// parseCues parses subtitle content of any supported format (srt, vtt, ass/ssa; the format is sniffed
// when the stated one does not parse) into cues with SRT-style timestamps. It never returns an
// empty successful result: zero cues is an error wrapping ErrInvalidSubtitleContent.
func (s *SubtitleService) parseCues(content, format string) ([]SubtitleLine, error) {
	content = strings.TrimPrefix(content, "\xef\xbb\xbf")
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return nil, fmt.Errorf("%w: not valid UTF-8 text", ErrInvalidSubtitleContent)
	}
	var lines []SubtitleLine
	var err error
	switch strings.ToLower(format) {
	case "vtt":
		lines, err = s.parseVTT(content)
	case "ass", "ssa":
		lines, err = s.parseASS(content)
	case "srt", "":
		lines, err = s.parseSRT(content)
	default:
		return nil, fmt.Errorf("%w: unsupported format %q", ErrInvalidSubtitleContent, format)
	}
	if err != nil || len(lines) == 0 {
		return nil, fmt.Errorf("%w: no valid %s cues found", ErrInvalidSubtitleContent, strings.ToUpper(format))
	}
	return lines, nil
}

// sniffSubtitleFormat guesses the format of stored content whose recorded format is unreliable.
func sniffSubtitleFormat(content string) string {
	t := strings.TrimSpace(strings.TrimPrefix(content, "\xef\xbb\xbf"))
	switch {
	case strings.HasPrefix(t, "WEBVTT"):
		return "vtt"
	case strings.Contains(t, "[Script Info]") || strings.Contains(t, "\nDialogue:"):
		return "ass"
	}
	return "srt"
}

type cueSpan struct{ start, end float64 }

func cueSpans(lines []SubtitleLine) []cueSpan {
	spans := make([]cueSpan, 0, len(lines))
	for _, l := range lines {
		st, e1 := parseTimestamp(l.StartTime)
		en, e2 := parseTimestamp(l.EndTime)
		if e1 != nil || e2 != nil || en < st {
			continue
		}
		spans = append(spans, cueSpan{st, en})
	}
	return spans
}

// resolveMediaFile returns the local file of a media item, or "" when it is not reachable locally.
// found=false means the media row itself does not exist.
func (s *SubtitleService) resolveMediaFile(ctx context.Context, mediaItemID int64) (path string, found bool, err error) {
	var fpath, name string
	var protocol, rootPath sql.NullString
	row := s.db.QueryRowContext(ctx, `SELECT f.path, f.name, sr.protocol, sr.path
		FROM files f LEFT JOIN storage_roots sr ON sr.id = f.storage_root_id WHERE f.id = ?`, mediaItemID)
	if err := row.Scan(&fpath, &name, &protocol, &rootPath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	var cands []string
	if protocol.Valid && protocol.String == "local" && rootPath.Valid && rootPath.String != "" {
		cands = append(cands, filepath.Join(rootPath.String, fpath), filepath.Join(rootPath.String, fpath, name))
	}
	cands = append(cands, fpath, filepath.Join(fpath, name))
	for _, c := range cands {
		if st, e := os.Stat(c); e == nil && st.Mode().IsRegular() {
			return c, true, nil
		}
	}
	return "", true, nil
}

func ffprobeDuration(ctx context.Context, file string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe",
		"-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", file).Output()
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("no duration")
	}
	return d, nil
}

var silenceRe = regexp.MustCompile(`silence_(start|end): (-?[0-9.]+)`)

// audioActivity returns the sorted non-silent intervals of the first analysis window of the media's audio.
func audioActivity(ctx context.Context, file string, duration float64) ([]cueSpan, error) {
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	limit := math.Min(duration, syncMaxAnalysis)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-protocol_whitelist", "file,pipe", "-t", fmt.Sprintf("%.3f", limit),
		"-i", file, "-vn", "-af", "silencedetect=noise=-35dB:d=0.3", "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if !strings.Contains(stderr.String(), "Audio:") {
		return nil, fmt.Errorf("no audio stream")
	}
	var silences []cueSpan
	var cur float64
	open := false
	for _, m := range silenceRe.FindAllStringSubmatch(stderr.String(), -1) {
		v, _ := strconv.ParseFloat(m[2], 64)
		if m[1] == "start" {
			cur, open = math.Max(v, 0), true
		} else if open {
			silences = append(silences, cueSpan{cur, v})
			open = false
		}
	}
	if open {
		silences = append(silences, cueSpan{cur, limit})
	}
	var active []cueSpan
	pos := 0.0
	for _, sl := range silences {
		if sl.start > pos {
			active = append(active, cueSpan{pos, sl.start})
		}
		pos = math.Max(pos, sl.end)
	}
	if pos < limit {
		active = append(active, cueSpan{pos, limit})
	}
	return active, nil
}

func toBins(spans []cueSpan, n int) []float64 {
	b := make([]float64, n)
	for _, sp := range spans {
		lo := int(math.Floor(sp.start / syncBinSeconds))
		hi := int(math.Ceil(sp.end / syncBinSeconds))
		for i := lo; i < hi; i++ {
			if i >= 0 && i < n {
				b[i] = 1
			}
		}
	}
	return b
}

// estimateDelay returns the delay d (seconds; positive = the subtitle runs LATE relative to the audio)
// maximising the Pearson correlation of cue(t+d) with audio-activity(t), together with the peak
// correlation and the best correlation at least 2 s away from the peak. ok=false when either signal is constant.
func estimateDelay(cues, audio []float64) (delay, peak, runnerUp float64, ok bool) {
	n := len(audio)
	if n == 0 || len(cues) == 0 {
		return 0, 0, 0, false
	}
	variance := func(x []float64) float64 {
		var s, s2 float64
		for _, v := range x {
			s += v
			s2 += v * v
		}
		m := s / float64(len(x))
		return s2/float64(len(x)) - m*m
	}
	if variance(audio) < 1e-9 || variance(cues) < 1e-9 {
		return 0, 0, 0, false
	}
	maxShift := int(syncMaxShiftSecs / syncBinSeconds)
	corr := make([]float64, 2*maxShift+1)
	for k := -maxShift; k <= maxShift; k++ {
		// pair audio[i] with cues[i+k]
		var sa, sc, saa, scc, sac float64
		cnt := 0
		for i := 0; i < n; i++ {
			j := i + k
			if j < 0 || j >= len(cues) {
				continue
			}
			a, c := audio[i], cues[j]
			sa += a
			sc += c
			saa += a * a
			scc += c * c
			sac += a * c
			cnt++
		}
		if cnt < n/2 {
			corr[k+maxShift] = -1
			continue
		}
		fc := float64(cnt)
		va, vc := saa/fc-(sa/fc)*(sa/fc), scc/fc-(sc/fc)*(sc/fc)
		if va < 1e-9 || vc < 1e-9 {
			corr[k+maxShift] = -1
			continue
		}
		corr[k+maxShift] = (sac/fc - (sa/fc)*(sc/fc)) / math.Sqrt(va*vc)
	}
	best := 0
	for i, v := range corr {
		if v > corr[best] {
			best = i
		}
	}
	runnerUp = -1
	far := int(2.0 / syncBinSeconds)
	for i, v := range corr {
		if subAbsInt(i-best) > far && v > runnerUp {
			runnerUp = v
		}
	}
	return float64(best-maxShift) * syncBinSeconds, corr[best], runnerUp, true
}

func subAbsInt(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

// verifySynchronization measures how well a subtitle track lines up with the real media.
// It never fabricates a result: when the media file cannot be analysed the status is
// "undetermined" with offset 0 and confidence 0.
func (s *SubtitleService) verifySynchronization(ctx context.Context, mediaItemID int64, track *SubtitleTrack) (*SubtitleSyncResult, error) {
	file, found, err := s.resolveMediaFile(ctx, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("resolve media: %w", err)
	}
	if !found {
		return nil, ErrMediaNotFound
	}

	content := ""
	if track.Content != nil {
		content = *track.Content
	}
	format := track.Format
	lines, perr := s.parseCues(content, format)
	if perr != nil {
		lines, perr = s.parseCues(content, sniffSubtitleFormat(content))
	}
	res := &SubtitleSyncResult{Status: SyncStatusUndetermined, Method: "none", SamplePoints: []SyncPoint{}}
	if perr != nil {
		res.Status = SyncStatusInvalid
		res.Reason = "the subtitle has no parseable cues"
		res.Recommendation = "Subtitle content is not a valid " + strings.ToUpper(format) + " file"
		return res, nil
	}
	spans := cueSpans(lines)
	res.DetectedFrames = len(spans)
	if len(spans) == 0 {
		res.Status = SyncStatusInvalid
		res.Reason = "the subtitle has no usable cue timings"
		res.Recommendation = res.Reason
		return res, nil
	}
	lastEnd := 0.0
	for _, sp := range spans {
		lastEnd = math.Max(lastEnd, sp.end)
	}

	duration := 0.0
	if file != "" {
		if d, derr := ffprobeDuration(ctx, file); derr == nil {
			duration = d
		}
	}
	if duration == 0 {
		if vi, verr := s.getVideoInfo(ctx, mediaItemID); verr == nil && vi.Duration > 0 {
			duration = vi.Duration
		}
	}
	res.MediaDuration = duration

	beyond := 0
	if duration > 0 {
		for _, sp := range spans {
			if sp.start > duration+syncDurationSlack {
				beyond++
			}
		}
	}
	if duration > 0 && lastEnd > duration+syncDurationSlack {
		res.Method = "duration"
		res.Reason = fmt.Sprintf("%d of %d cues start beyond the media duration (%.1fs); last cue ends at %.1fs", beyond, len(spans), duration, lastEnd)
	}

	// Measurement against the real audio.
	if file == "" {
		if res.Reason == "" {
			res.Reason = "the media file is not readable locally, so synchronisation cannot be measured"
		}
	} else if duration == 0 {
		if res.Reason == "" {
			res.Reason = "the media duration could not be determined"
		}
	} else {
		active, aerr := audioActivity(ctx, file, duration)
		if aerr != nil {
			if res.Reason == "" {
				res.Reason = "the audio of the media file could not be analysed"
			}
		} else {
			n := int(math.Min(duration, syncMaxAnalysis) / syncBinSeconds)
			cueBins := toBins([]cueSpan(spans), int(math.Min(math.Max(lastEnd, duration)+syncMaxShiftSecs, syncMaxAnalysis+3*syncMaxShiftSecs)/syncBinSeconds)+1)
			delay, peak, runner, ok := estimateDelay(cueBins, toBins(active, n))
			switch {
			case !ok:
				if res.Reason == "" {
					res.Reason = "the audio has no speech-activity variation to align against"
				}
			case peak < syncMinPeak || peak-runner < syncMinMargin:
				if res.Reason == "" {
					res.Reason = fmt.Sprintf("no unambiguous alignment with the audio (best correlation %.2f)", peak)
				}
			default:
				res.Method = "audio-activity"
				res.SyncOffset = delay * 1000
				res.Confidence = math.Min(1, peak-math.Max(runner, 0))
				if res.Confidence < 0 {
					res.Confidence = 0
				}
				if res.Status != SyncStatusInvalid {
					res.Status = SyncStatusMeasured
				}
				for _, sp := range spans {
					res.SamplePoints = append(res.SamplePoints, SyncPoint{
						SubtitleTime: sp.start, VideoTime: sp.start - delay,
						Text: "", Confidence: res.Confidence,
					})
				}
				res.Reason = ""
			}
		}
	}

	switch {
	case beyond > 0 && duration > 0 && lastEnd > duration+syncDurationSlack:
		res.IsValid = false
		if res.Status != SyncStatusMeasured {
			res.Status = SyncStatusInvalid
		} else if res.Reason == "" {
			res.Reason = fmt.Sprintf("%d of %d cues start beyond the media duration", beyond, len(spans))
		}
		res.Recommendation = "Subtitle runs past the end of the media; it does not belong to this file or is badly shifted"
	case res.Status == SyncStatusMeasured && math.Abs(res.SyncOffset) <= syncValidOffsetMs:
		res.IsValid = true
		res.Recommendation = "Subtitle synchronization is good"
	case res.Status == SyncStatusMeasured:
		res.Recommendation = fmt.Sprintf("Subtitle is %.1fs %s relative to the audio; shift it by %+.1fs",
			math.Abs(res.SyncOffset)/1000, map[bool]string{true: "late", false: "early"}[res.SyncOffset > 0], -res.SyncOffset/1000)
	default:
		res.Recommendation = "Synchronization could not be determined: " + res.Reason
	}
	return res, nil
}
