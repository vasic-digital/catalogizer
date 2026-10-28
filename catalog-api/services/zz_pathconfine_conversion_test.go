package services

import (
	"testing"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §11.4.115: absolute source/target paths outside the configured roots gave arbitrary
// file read (convert→download) and arbitrary write (ffmpeg/ebook/text output).
func TestPathConfine_ConversionRejectsPathsOutsideRoots(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CATALOGIZER_CONVERSION_ROOTS", root)
	db := setupTestDB(t)
	svc := NewConversionService(repository.NewConversionRepository(db), nil, nil)

	mk := func(src, dst string) *models.ConversionRequest {
		return &models.ConversionRequest{SourcePath: src, TargetPath: dst, SourceFormat: "png", TargetFormat: "jpg", ConversionType: "image"}
	}
	_, err := svc.CreateConversionJob(1, mk(root+"/in.png", root+"/out.jpg"))
	require.NoError(t, err, "control: paths inside root accepted")

	for name, req := range map[string]*models.ConversionRequest{
		"source /etc":     mk("/etc/passwd", root+"/out.jpg"),
		"target ssh keys": mk(root+"/in.png", "/root/.ssh/authorized_keys"),
		"both outside":    mk("/etc/shadow", "/tmp/stolen.jpg"),
		"sibling prefix":  mk(root+"-evil/in.png", root+"/out.jpg"),
		"relative":        mk("in.png", "out.jpg"),
	} {
		_, err := svc.CreateConversionJob(1, req)
		assert.Error(t, err, name)
	}
}
