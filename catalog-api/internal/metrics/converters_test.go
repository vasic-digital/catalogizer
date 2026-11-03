package metrics

import "testing"

func TestProbeConverters_RealLookups(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no tools on PATH
	got := ProbeConverters()
	if len(got) != len(ConverterTools) {
		t.Fatalf("want an entry per tool, got %v", got)
	}
	for tool, ok := range got {
		if ok {
			t.Errorf("%s reported present with an empty PATH", tool)
		}
	}
}
