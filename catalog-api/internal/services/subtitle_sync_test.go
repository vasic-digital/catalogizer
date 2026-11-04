package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestEstimateDelay_FindsKnownShiftAndRejectsConstantSignals(t *testing.T) {
	n := 1200
	audio := make([]float64, n)
	for _, b := range [][2]int{{20, 45}, {60, 72}, {100, 130}, {150, 160}, {190, 225}, {250, 262}, {290, 330}} {
		for i := b[0]; i < b[1]; i++ {
			audio[i] = 1
		}
	}
	const shiftBins = 70 // subtitle late by 7.0 s
	cues := make([]float64, n+400)
	for i, v := range audio {
		cues[i+shiftBins] = v
	}
	d, peak, _, ok := estimateDelay(cues, audio)
	require.True(t, ok)
	assert.InDelta(t, 7.0, d, 0.15)
	assert.Greater(t, peak, 0.9)

	_, _, _, ok = estimateDelay(cues, make([]float64, n)) // silent audio: nothing to align against
	assert.False(t, ok)
	all := make([]float64, n)
	for i := range all {
		all[i] = 1
	}
	_, _, _, ok = estimateDelay(cues, all) // always-active audio
	assert.False(t, ok)
}

func TestParseCues_AllFormatsAndRejections(t *testing.T) {
	s := &SubtitleService{logger: zap.NewNop()}
	srt := "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n\r\n"
	vtt := "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n\n"
	ass := "[Script Info]\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hello\n"
	for name, c := range map[string][2]string{"srt": {srt, "srt"}, "vtt": {vtt, "vtt"}, "ass": {ass, "ass"}} {
		lines, err := s.parseCues(c[0], c[1])
		require.NoError(t, err, name)
		require.Len(t, lines, 1, name)
		st, _ := parseTimestamp(lines[0].StartTime)
		assert.InDelta(t, 1.0, st, 0.001, name)
	}
	for _, bad := range []string{"", "plain text", "\xff\xfe\x00\x01", "WEBVTT\n"} {
		_, err := s.parseCues(bad, "srt")
		assert.True(t, errors.Is(err, ErrInvalidSubtitleContent), "%q", bad)
	}
}
