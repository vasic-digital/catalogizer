package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProbeConverters_RealLookups(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no tools on PATH
	got := ProbeConverters()
	if len(got) != len(ConverterTools)+2 {
		t.Fatalf("want an entry per tool plus %s and %s, got %v", ConverterPDFRaster, ConverterSVGRender, got)
	}
	for tool, ok := range got {
		if ok {
			t.Errorf("%s reported present with an empty PATH", tool)
		}
	}
}

// stubTool puts an executable (or, when exec is false, a non-executable) file named name in dir.
// The probe only resolves names on PATH (exec.LookPath), it never runs the tool, so a stub file
// is exactly what is needed to prove the "present" side of the lookup.
func stubTool(t *testing.T, dir, name string, exec bool) {
	t.Helper()
	mode := os.FileMode(0o644)
	if exec {
		mode = 0o755
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), mode); err != nil {
		t.Fatal(err)
	}
}

func TestProbeConverters_ToolPresentSide(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	stubTool(t, dir, "ffmpeg", true)
	got := ProbeConverters()
	if !got["ffmpeg"] || got["convert"] || got["pdftotext"] || got[ConverterPDFRaster] {
		t.Fatalf("only ffmpeg is on PATH, got %v", got)
	}

	stubTool(t, dir, "convert", true)
	stubTool(t, dir, "pdftotext", true)
	got = ProbeConverters()
	for _, tool := range ConverterTools {
		if !got[tool] {
			t.Errorf("%s is on PATH but reported absent: %v", tool, got)
		}
	}
	if got[ConverterPDFRaster] {
		t.Errorf("%s must stay false without gs: convert alone cannot rasterise a PDF: %v", ConverterPDFRaster, got)
	}

	stubTool(t, dir, "gs", true)
	if got = ProbeConverters(); !got[ConverterPDFRaster] {
		t.Errorf("convert and gs are on PATH but %s is false: %v", ConverterPDFRaster, got)
	}

	// SVG input: convert alone is not enough, a renderer is needed (F5 of review round 1)
	if got[ConverterSVGRender] {
		t.Errorf("%s must stay false without inkscape/rsvg-convert: %v", ConverterSVGRender, got)
	}
	stubTool(t, dir, "rsvg-convert", true)
	if got = ProbeConverters(); !got[ConverterSVGRender] {
		t.Errorf("convert and rsvg-convert are on PATH but %s is false: %v", ConverterSVGRender, got)
	}
	if err := os.Remove(filepath.Join(dir, "rsvg-convert")); err != nil {
		t.Fatal(err)
	}
	stubTool(t, dir, "inkscape", true)
	if got = ProbeConverters(); !got[ConverterSVGRender] {
		t.Errorf("convert and inkscape are on PATH but %s is false: %v", ConverterSVGRender, got)
	}

	// gs without convert is not PDF-capable either
	if err := os.Remove(filepath.Join(dir, "convert")); err != nil {
		t.Fatal(err)
	}
	if got = ProbeConverters(); got[ConverterPDFRaster] || got["convert"] {
		t.Errorf("convert is gone: %v", got)
	}
	if got[ConverterSVGRender] {
		t.Errorf("a renderer without convert cannot convert SVG: %v", got)
	}
}

func TestProbeConverters_NonExecutableFileIsAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	stubTool(t, dir, "ffmpeg", false) // a file named ffmpeg without the exec bit is not a usable tool
	if ProbeConverters()["ffmpeg"] {
		t.Fatal("a non-executable file was reported as an installed tool")
	}
}
