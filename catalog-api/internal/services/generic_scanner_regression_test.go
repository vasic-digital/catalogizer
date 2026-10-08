package services

import (
	"context"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// PA-03 regression (T361/T362): the FTP, NFS and WebDAV scanners used to be empty bodies returning nil, so a scan of a populated root
// ended "completed" with 0 files. These three tests drive the scanners THROUGH THEIR REAL CONSTRUCTORS and the real ProtocolScanner interface.

func seededTree() *memFS {
	m := newMemFS()
	m.addDir("/")
	m.addFile("/", "a.mkv", 100, t0)
	m.addSubdir("/", "music")
	m.addFile("/music", "b.flac", 200, t0)
	m.addSubdir("/music", "album")
	m.addFile("/music/album", "c.mp3", 300, t0)
	return m
}

func scanJobFor(protocol string) ScanJob {
	return ScanJob{ID: "job-" + protocol, StorageRoot: &models.StorageRoot{Name: "root-" + protocol, Protocol: protocol}, Path: "/", MaxDepth: 10, ScanType: "full", Context: context.Background()}
}

func TestStubScannersNowScanPopulatedTree(t *testing.T) {
	cases := map[string]ProtocolScanner{
		"ftp":    NewFTPScanner(zap.NewNop()),
		"nfs":    NewNFSScanner(zap.NewNop()),
		"webdav": NewWebDAVScanner(zap.NewNop()),
	}
	for proto, sc := range cases {
		proto, sc := proto, sc
		t.Run(proto, func(t *testing.T) {
			st := &ScanStatus{JobID: "j", Protocol: proto}
			err := sc.ScanPath(context.Background(), seededTree(), scanJobFor(proto), st)
			require.NoError(t, err)
			snap := st.GetSnapshot()
			assert.Equal(t, int64(5), snap.FilesFound, "3 files + 2 directories must be found, a scan that returns nil without looking is the PASS-bluff")
		})
	}
}
