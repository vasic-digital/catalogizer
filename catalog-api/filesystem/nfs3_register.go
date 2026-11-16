package filesystem

import (
	"context"
	"fmt"
	"net"
	"sync"

	"digital.vasic.filesystem/pkg/nfs3"
)

// Settings keys consumed only by the nfs3 protocol (the user-space, read-only, rootless NFSv3 client of pkg/nfs3).
const (
	// KeyUID and KeyGID are the AUTH_SYS identity sent to the server. Absent or 0 means the client default (65534, nobody), never root.
	KeyUID = "uid"
	KeyGID = "gid"
	// KeyMountPort and KeyPortmapPort are optional per-service ports for servers that run MOUNT / the portmapper off the NFS port. Each
	// defaults to nothing of its own: mount_port falls back to port, portmap_port to the client default (111).
	KeyMountPort   = "mount_port"
	KeyPortmapPort = "portmap_port"
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
		// host = server, path = exported path (the export to MOUNT), port = the NFS port and, unless mount_port is set, the MOUNT port when
		// the server has no portmapper (absent: ask the portmapper), mount_port / portmap_port = optional split ports, uid / gid = AUTH_SYS identity.
		Keys: map[string]KeyType{
			KeyHost: KeyString, KeyPort: KeyInt, KeyPath: KeyString, KeyUID: KeyInt, KeyGID: KeyInt,
			KeyMountPort: KeyInt, KeyPortmapPort: KeyInt,
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
	mountPort := IntSetting(settings, KeyMountPort, port)
	portmapPort := IntSetting(settings, KeyPortmapPort, 0)
	// Contract (NFS3W-FU-m1): every port key is 0-65535 and 0 means "default" (port: ask the portmapper; mount_port: follows port; portmap_port: 111).
	// `port` is checked first and by name, so an out-of-range port is reported as port, not as the mount_port it would be copied into.
	for _, p := range []struct {
		name string
		v    int
	}{{KeyPort, port}, {KeyMountPort, mountPort}, {KeyPortmapPort, portmapPort}} {
		if p.v < 0 || p.v > 65535 {
			return nfs3.Config{}, fmt.Errorf("nfs3: %s must be 0-65535 (0 = default)", p.name)
		}
	}
	return nfs3.Config{
		Host: host, Export: export,
		MountPort: mountPort, NFSPort: port, PortmapPort: portmapPort,
		UID: uint32(uid), GID: uint32(gid),
		// SEC-DIAL-1: every connection (portmapper, MOUNT, NFS) goes through the operator-class dialer, which re-resolves and re-checks the address class on every dial; the
		// closure reads storageDialContext at dial time so tests can observe or replace it. The reserved-port option is never used.
		Dial: func(ctx context.Context, network, addr string, _ bool) (net.Conn, error) {
			return storageDial(ctx, network, addr)
		},
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
