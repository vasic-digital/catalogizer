package services

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"catalogizer/models"
)

// D11a: ffmpeg exits 0 on a truncated source when the decoder only logs "Invalid data" and the
// muxer finishes what it got, so the job used to end `completed` with a tiny output and no hint.
// After a successful audio/video run the output duration is compared with the duration the
// SOURCE container declares. A clearly shorter output is not a failure (a usable file exists)
// but it is flagged on the job (`warning`) so the owner is not told it all went fine.
//
// Honest limits: the source duration is the container's own claim, so a source whose header
// already matches the truncated payload (or whose duration is only estimated from file size,
// as for some MP3/MPEG-TS files) is not detected; a source or output that ffprobe cannot read
// yields no warning (nothing is claimed either way).
const (
	truncationRatio     = 0.9 // output below 90 % of the declared source duration
	truncationMinGapSec = 1.0 // and at least this many seconds shorter
	probeTimeout        = 30 * time.Second
)

// probeMediaDuration reads format.duration through the hardened launcher (prlimit, protocol and
// demuxer whitelists). ok is false when the duration is unknown.
func probeMediaDuration(ctx context.Context, path string) (secs float64, ok bool) {
	bin, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, false
	}
	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cmd, err := hardenedCommand(pctx, "ffprobe", bin,
		[]string{"-show_entries", "format=duration", "-of", "csv=p=0", "file:" + path})
	if err != nil {
		return 0, false
	}
	var out bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, &out, nil
	if err := cmd.Run(); err != nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(out.String()), 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// truncationWarning returns the client-safe warning for a finished audio/video job, or "".
func (s *ConversionService) truncationWarning(job *models.ConversionJob) string {
	if job.ConversionType != models.ConversionTypeVideo && job.ConversionType != models.ConversionTypeAudio {
		return ""
	}
	ctx := s.runCtxFor(job)
	src, ok := probeMediaDuration(ctx, job.SourcePath)
	if !ok {
		return ""
	}
	out, ok := probeMediaDuration(ctx, job.TargetPath)
	if !ok {
		return ""
	}
	if out < src*truncationRatio && src-out >= truncationMinGapSec {
		return fmt.Sprintf("The output is %.1f s but the source declares %.1f s: the output is much shorter than the source, which usually means the source file is truncated or damaged. Check the result before relying on it.", out, src)
	}
	return ""
}
