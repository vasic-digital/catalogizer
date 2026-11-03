package models

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func qj(w, h int) string {
	return fmt.Sprintf(`{"resolution":{"width":%d,"height":%d}}`, w, h)
}

// CAT-001 review F11 (M4/M5): exact bucket boundaries.
func TestCAT001_QualityBucketBoundaries(t *testing.T) {
	cases := []struct {
		name string
		js   string
		dvd  bool
		want string
	}{
		{"4k by height", qj(100, 2160), false, "4K/UHD"},
		{"4k by width", qj(3840, 100), false, "4K/UHD"},
		{"just under 4k is 1080p", qj(3839, 2159), false, "1080p"},
		{"1080p by height", qj(10, 1080), false, "1080p"},
		{"1080p by width", qj(1920, 10), false, "1080p"},
		{"720p by height", qj(10, 720), false, "720p"},
		{"720p by width", qj(1280, 10), false, "720p"},
		{"under 720 no dvd", qj(640, 719), false, "Other"},
		{"480 with dvd", qj(640, 480), true, "DVD"},
		{"719 with dvd", qj(640, 719), true, "DVD"},
		{"480 without dvd", qj(640, 480), false, "Other"},
		{"479 with dvd", qj(640, 479), true, "Other"},
		{"empty", "", true, "Other"},
		{"invalid json", "{", true, "Other"},
		{"no resolution", `{}`, true, "Other"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, QualityBucket(c.js, c.dvd), c.name)
	}
}
