package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ftpTempName = ".movie.mkv.ftpcopy-0123456789abcdef.part"

func ftpTempTree(proto string) *memFS {
	m := newMemFS()
	m.proto = proto
	m.addDir("/")
	m.addFile("/", "movie.mkv", 10, t0)
	m.addFile("/", ftpTempName, 5, t0)                               // a CopyFile staging file
	m.addFile("/", ".x.part", 5, t0)                                 // a user's look-alike: must be catalogued
	m.addFile("/", "movie.mkv.ftpcopy-0123456789abcdef.part", 5, t0) // no leading dot: not a temp
	m.addSubdir("/", ".dir.ftpcopy-0123456789abcdef.part")           // a directory with a temp-like name: not a temp
	m.addFile("/sub", ftpTempName, 5, t0)
	m.addSubdir("/", "sub")
	return m
}

// FTP storage: the staging files of CopyFile are not catalogue entries (FTPClient.ListDirectory reports them, the scanner hides them).
func TestGenericScanner_FTPCopyTempFilesAreNotCatalogued(t *testing.T) {
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, nil), ftpTempTree("ftp"), jobOf("/", 10, "full"))
	require.NoError(t, err)
	got := rec.sorted()
	assert.NotContains(t, got, "/"+ftpTempName)
	assert.NotContains(t, got, "/sub/"+ftpTempName)
	assert.Contains(t, got, "/movie.mkv")
	assert.Contains(t, got, "/.x.part", "a user's own look-alike file is still catalogued")
	assert.Contains(t, got, "/movie.mkv.ftpcopy-0123456789abcdef.part", "without the leading dot it is not a copy temp")
	assert.Contains(t, got, "/.dir.ftpcopy-0123456789abcdef.part", "a directory is never a copy temp")
}

// Other protocols never get this filter: the same name is an ordinary file there.
func TestGenericScanner_CopyTempNameIsOrdinaryOnOtherProtocols(t *testing.T) {
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, nil), ftpTempTree("smb"), jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Contains(t, rec.sorted(), "/"+ftpTempName)
}
