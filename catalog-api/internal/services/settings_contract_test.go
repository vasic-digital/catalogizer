package services

import (
	"testing"

	"catalogizer/filesystem"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// PA-01 settings contract: whatever storageRootToSettings writes, the client factory must (a) accept and (b) deliver to the
// client. Before PA-01 the NFS export went out as "export_path" while the factory read "path" (always ""), and the FTP and
// WebDAV "path" was never forwarded at all, so a scan of a sub-tree silently scanned the wrong place.

func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }

func contractRoot(protocol string) *models.StorageRoot {
	r := &models.StorageRoot{Name: "contract-" + protocol, Protocol: protocol}
	switch protocol {
	case "local":
		r.Path = sp("/data/local-root")
	case "smb":
		r.Host, r.Port, r.Path = sp("smb.example"), ip(1445), sp("sharename")
		r.Username, r.Password, r.Domain = sp("smbuser"), sp("smbpass"), sp("DOM")
	case "ftp":
		r.Host, r.Port, r.Path = sp("ftp.example"), ip(2121), sp("/pub/media")
		r.Username, r.Password = sp("ftpuser"), sp("ftppass")
	case "nfs":
		r.Host, r.Path, r.MountPoint, r.Options = sp("nfs.example"), sp("/export/media"), sp("/mnt/contract-nfs"), sp("vers=3,ro")
	case "webdav":
		r.URL, r.Path = sp("https://dav.example:8443/dav"), sp("/media")
		r.Username, r.Password = sp("davuser"), sp("davpass")
	}
	return r
}

func TestSettingsContract_ScannerSettingsReachTheClient(t *testing.T) {
	factory := filesystem.NewDefaultClientFactory()
	scanner := &UniversalScanner{logger: zap.NewNop()}
	for _, proto := range []string{"local", "smb", "ftp", "nfs", "webdav"} {
		proto := proto
		t.Run(proto, func(t *testing.T) {
			root := contractRoot(proto)
			settings := scanner.storageRootToSettings(root)
			c, err := factory.CreateClient(&filesystem.StorageConfig{ID: root.Name, Name: root.Name, Protocol: proto, Settings: settings})
			require.NoError(t, err, "the factory must accept every key the scanner writes")
			switch proto {
			case "local":
				assert.Equal(t, "/data/local-root", c.GetConfig().(*filesystem.LocalConfig).BasePath)
			case "smb":
				cfg := c.GetConfig().(*filesystem.SmbConfig)
				assert.Equal(t, "smb.example", cfg.Host)
				assert.Equal(t, 1445, cfg.Port)
				assert.Equal(t, "sharename", cfg.Share)
				assert.Equal(t, "smbuser", cfg.Username)
				assert.Equal(t, "DOM", cfg.Domain)
			case "ftp":
				cfg := c.GetConfig().(*filesystem.FTPConfig)
				assert.Equal(t, "ftp.example", cfg.Host)
				assert.Equal(t, 2121, cfg.Port)
				assert.Equal(t, "/pub/media", cfg.Path, "the FTP root path must reach the client")
			case "nfs":
				cfg := c.GetConfig().(*filesystem.NFSConfig)
				assert.Equal(t, "nfs.example", cfg.Host)
				assert.Equal(t, "/export/media", cfg.Path, "the NFS export must reach the client (was export_path vs path)")
				assert.Equal(t, "/mnt/contract-nfs", cfg.MountPoint)
				assert.Equal(t, "vers=3,ro", cfg.Options)
			case "webdav":
				cfg := c.GetConfig().(*filesystem.WebDAVConfig)
				assert.Equal(t, "https://dav.example:8443/dav", cfg.URL)
				assert.Equal(t, "/media", cfg.Path, "the WebDAV path must reach the client")
			}
		})
	}
}
