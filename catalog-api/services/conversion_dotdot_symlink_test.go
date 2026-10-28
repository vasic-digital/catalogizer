package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Adopted from the review-round-3 reviewer: ".." in the REQUEST path after a symlink is cleaned lexically by Confine
// (filepath.Clean before resolve) while the conversion writes the raw path through the kernel.
func TestConversion_DotDotAfterSymlinkDoesNotEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside", "sub")
	_ = os.MkdirAll(root, 0o755)
	_ = os.MkdirAll(outside, 0o755)
	if err := os.Symlink(outside, filepath.Join(root, "lnk")); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "in.txt")
	_ = os.WriteFile(src, []byte("x"), 0o644)
	t.Setenv("CATALOGIZER_CONVERSION_ROOTS", root)
	t.Setenv("CATALOGIZER_READ_ROOTS", root)
	target := root + "/lnk/../pwn.txt"
	s := &ConversionService{}
	ok := s.validateConversionRequest(&models.ConversionRequest{SourcePath: src, TargetPath: target, SourceFormat: "txt", TargetFormat: "html", ConversionType: models.ConversionTypeDocument})
	err := confineConversionPaths(src, target)
	f, werr := os.Create(target)
	if werr == nil {
		f.Close()
	}
	_, statOut := os.Stat(filepath.Join(base, "outside", "pwn.txt"))
	t.Logf("validate=%v confineErr=%v kernelWroteOutside=%v", ok, err, statOut == nil)
	if err == nil && statOut == nil {
		t.Fatalf("ESCAPE: confine accepted %q, kernel wrote %s", target, filepath.Join(base, "outside", "pwn.txt"))
	}
	// The refusal must come from the path confinement itself. validateConversionRequest calls
	// confineConversionPaths, so it must refuse too (before the fix both returned "allowed").
	if err == nil {
		t.Fatalf("confineConversionPaths accepted %q (the kernel resolves it to %s)", target, filepath.Join(base, "outside", "pwn.txt"))
	}
	if ok {
		t.Fatalf("validateConversionRequest accepted %q", target)
	}
}

// End to end: the public entry points (job creation, and the run-time recheck of a job stored
// before the fix) refuse the same request; the control request with the same symlink but no
// ".." is accepted, so the refusal is caused by the ".." and not by the fixture.
func TestConversion_DotDotAfterSymlinkRefusedEndToEnd(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside", "sub")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "lnk")))
	src := filepath.Join(root, "in.txt")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o644))
	t.Setenv(pathGuardEnvConv, root)
	t.Setenv(pathGuardEnvRead, root)

	db := setupTestDB(t)
	repo := repository.NewConversionRepository(db)
	svc := NewConversionService(repo, nil, nil)
	req := func(dst string) *models.ConversionRequest {
		return &models.ConversionRequest{SourcePath: src, TargetPath: dst, SourceFormat: "txt", TargetFormat: "html", ConversionType: models.ConversionTypeDocument}
	}

	_, err := svc.CreateConversionJob(1, req(filepath.Join(root, "ok.html")))
	require.NoError(t, err, "control: an ordinary target inside the root is accepted")

	evil := root + "/lnk/../pwn.html"
	_, err = svc.CreateConversionJob(1, req(evil))
	assert.Error(t, err, "CreateConversionJob must refuse %q", evil)

	// a job stored before the fix is refused when it runs, and is marked failed
	job := &models.ConversionJob{UserID: 1, SourcePath: src, TargetPath: evil,
		SourceFormat: "txt", TargetFormat: "html", ConversionType: models.ConversionTypeDocument,
		Status: models.ConversionStatusPending, CreatedAt: time.Now()}
	id, err := repo.CreateJob(job)
	require.NoError(t, err)
	assert.Error(t, svc.StartConversion(id))
	got, err := repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusFailed, got.Status)
	_, statErr := os.Stat(filepath.Join(base, "outside", "pwn.html"))
	assert.True(t, os.IsNotExist(statErr), "nothing may be written outside the root")
}
