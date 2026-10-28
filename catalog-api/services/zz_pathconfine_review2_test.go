package services

import (
	"path/filepath"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
)

// Review round 2 (T1, T2): read roots and write roots must be told apart for every field.
func TestPathConfine_Review2_ReadVersusWriteDiscrimination(t *testing.T) {
	media := t.TempDir()     // read root only (like /media mounted :ro)
	writeRoot := t.TempDir() // sync write root
	t.Setenv(pathGuardEnvRead, media)
	t.Setenv(pathGuardEnvSync, writeRoot)
	svc, _ := newSyncSvc(t)

	mk := func(typ, dir, local, settings string) *models.SyncEndpoint {
		e := &models.SyncEndpoint{Name: "n", Type: typ, URL: "http://x", SyncDirection: dir, LocalPath: local, RemotePath: "/r"}
		if typ == models.SyncTypeLocal {
			e.RemotePath = media
		}
		if settings != "" {
			e.SyncSettings = &settings
		}
		return e
	}

	// T1: local endpoint, sync_mode=bidirectional, NO source_directory: the local driver
	// then uses local_path as the source side and WRITES it, so local_path must be in a
	// write root even though sync_direction=upload would otherwise only read it.
	bidiNoSrc := `{"destination_directory":"` + filepath.Join(writeRoot, "d") + `","sync_mode":"bidirectional"}`
	assert.Error(t, svc.validateSyncEndpoint(mk(models.SyncTypeLocal, models.SyncDirectionUpload, filepath.Join(media, "x"), bidiNoSrc)),
		"T1: bidirectional local sync without source_directory writes local_path; a read-only root must be refused")
	assert.NoError(t, svc.validateSyncEndpoint(mk(models.SyncTypeLocal, models.SyncDirectionUpload, filepath.Join(writeRoot, "x"), bidiNoSrc)),
		"T1 control: the same endpoint with local_path in a write root is accepted")
	oneWayNoSrc := `{"destination_directory":"` + filepath.Join(writeRoot, "d") + `","sync_mode":"mirror"}`
	assert.NoError(t, svc.validateSyncEndpoint(mk(models.SyncTypeLocal, models.SyncDirectionUpload, filepath.Join(media, "x"), oneWayNoSrc)),
		"T1 control: mirror local sync only READS local_path, so the read root is accepted")

	// P4 (documented rule): a local mirror whose source is local_path (no source_directory)
	// and whose sync_direction is download/bidirectional is refused for a read root, because
	// local_path is classed by sync_direction (download/bidirectional = written). Setting
	// source_directory, or sync_direction=upload, is the supported way to mirror from /media.
	assert.Error(t, svc.validateSyncEndpoint(mk(models.SyncTypeLocal, models.SyncDirectionDownload, filepath.Join(media, "x"), oneWayNoSrc)),
		"P4: download-direction local endpoint with local_path in a read root is refused")
	withSrc := `{"source_directory":"` + filepath.Join(media, "x") + `","destination_directory":"` + filepath.Join(writeRoot, "d") + `","sync_mode":"mirror"}`
	assert.NoError(t, svc.validateSyncEndpoint(mk(models.SyncTypeLocal, models.SyncDirectionDownload, filepath.Join(writeRoot, "x"), withSrc)),
		"P4 control: the same mirror with source_directory in the read root is accepted")

	// T2: destination_directory is a WRITE target for every type: a directory inside a
	// read root (but outside the write roots) is refused, one inside a write root accepted.
	// source_directory is only READ (unless bidirectional): a read root is accepted.
	for _, typ := range []string{models.SyncTypeWebDAV, models.SyncTypeCloudStorage} {
		local := filepath.Join(media, "m")
		dstInRead := `{"destination_directory":"` + filepath.Join(media, "d") + `"}`
		assert.Error(t, svc.validateSyncEndpoint(mk(typ, models.SyncDirectionUpload, local, dstInRead)),
			"T2 %s: destination_directory in a read-only root must be refused", typ)
		dstInWrite := `{"destination_directory":"` + filepath.Join(writeRoot, "d") + `"}`
		assert.NoError(t, svc.validateSyncEndpoint(mk(typ, models.SyncDirectionUpload, local, dstInWrite)),
			"T2 %s: destination_directory in a write root must be accepted", typ)
		srcInRead := `{"source_directory":"` + filepath.Join(media, "s") + `"}`
		assert.NoError(t, svc.validateSyncEndpoint(mk(typ, models.SyncDirectionUpload, local, srcInRead)),
			"T2 %s: source_directory in a read root is readable and must be accepted", typ)
		srcInReadBidi := `{"source_directory":"` + filepath.Join(media, "s") + `","sync_mode":"bidirectional"}`
		assert.Error(t, svc.validateSyncEndpoint(mk(typ, models.SyncDirectionUpload, local, srcInReadBidi)),
			"T2 %s: bidirectional source_directory is written; a read root must be refused", typ)
	}
}
