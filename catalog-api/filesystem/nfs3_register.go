package filesystem

import (
	"fmt"
	"sync"

	"digital.vasic.filesystem/pkg/nfs3"
)

// Settings keys consumed only by the nfs3 protocol (the user-space, read-only, rootless NFSv3 client of pkg/nfs3).
const (
	// KeyUID and KeyGID are the AUTH_SYS identity sent to the server. Absent or 0 means the client default (65534, nobody), never root.
	KeyUID = "uid"
	KeyGID = "gid"
)

var nfs3RegisterMu sync.Mutex

// RegisterNFS3Protocol makes "nfs3" a creatable storage protocol (factory, SupportedProtocols, generic scanner). Like RegisterS3Protocol it is
// explicit, not an init(); main() calls it once at start-up. Safe to call more than once.
func RegisterNFS3Protocol() error {
	nfs3RegisterMu.Lock()
	defer nfs3RegisterMu.Unlock()
	if IsRegisteredProtocol("nfs3") {
		return nil
	}
	return RegisterProtocol(ProtocolSpec{
		Name: "nfs3",
		// host = server, path = exported path (the export to MOUNT), port = the NFS and MOUNT port when the server has no portmapper
		// (absent: ask the portmapper), uid / gid = AUTH_SYS identity.
		Keys: map[string]KeyType{
			KeyHost: KeyString, KeyPort: KeyInt, KeyPath: KeyString, KeyUID: KeyInt, KeyGID: KeyInt,
		},
		Required: []string{KeyHost, KeyPath},
		New:      newNFS3FromSettings,
	})
}

// nfs3ConfigFromSettings is the Config mapping: validated settings -> pkg/nfs3 Config. It does not dial.
func nfs3ConfigFromSettings(settings map[string]interface{}) (nfs3.Config, error) {
	host := StringSetting(settings, KeyHost, "")
	if host == "" {
		return nfs3.Config{}, fmt.Errorf("nfs3: host is required")
	}
	export := StringSetting(settings, KeyPath, "")
	if export == "" {
		return nfs3.Config{}, fmt.Errorf("nfs3: path (the exported path) is required")
	}
	uid, gid := IntSetting(settings, KeyUID, 0), IntSetting(settings, KeyGID, 0)
	if uid < 0 || gid < 0 {
		return nfs3.Config{}, fmt.Errorf("nfs3: uid and gid must not be negative")
	}
	port := IntSetting(settings, KeyPort, 0)
	return nfs3.Config{
		Host: host, Export: export,
		MountPort: port, NFSPort: port,
		UID: uint32(uid), GID: uint32(gid),
	}, nil
}

// newNFS3FromSettings builds the client. The caller owns the lifecycle: Connect before use (portmapper -> MNT -> FSINFO -> GETATTR root),
// Disconnect when done (best-effort UMNT, connections closed). Nothing is dialled here.
func newNFS3FromSettings(settings map[string]interface{}) (FileSystemClient, error) {
	cfg, err := nfs3ConfigFromSettings(settings)
	if err != nil {
		return nil, err
	}
	c, err := nfs3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create NFS3 client: %w", err)
	}
	return c, nil
}
