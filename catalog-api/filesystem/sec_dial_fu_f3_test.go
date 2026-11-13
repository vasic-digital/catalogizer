package filesystem

import (
	"context"
	"errors"
	"net"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// F3: the NFS server resolution is ONE shared function (Linux and darwin): a policy refusal is ErrStorageHostRefused, an unresolvable host is
// "does not resolve" (not a refusal).
func TestSECDIALFU_F3_ResolveNFSServerClassifies(t *testing.T) {
	old := storageOperatorResolve
	defer func() { storageOperatorResolve = old }()

	storageOperatorResolve = func(ctx context.Context, h string) ([]net.IP, error) { return nil, errors.New("lookup failed") }
	_, err := resolveNFSServer(context.Background(), "nas.lan")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrStorageHostRefused)
	require.Contains(t, err.Error(), "does not resolve")

	storageOperatorResolve = func(ctx context.Context, h string) ([]net.IP, error) { return nil, nil }
	_, err = resolveNFSServer(context.Background(), "nas.lan")
	require.Contains(t, err.Error(), "does not resolve")

	storageOperatorResolve = func(ctx context.Context, h string) ([]net.IP, error) { return nil, netguard.ErrBlocked }
	_, err = resolveNFSServer(context.Background(), "nas.lan")
	require.ErrorIs(t, err, ErrStorageHostRefused)

	storageOperatorResolve = func(ctx context.Context, h string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.1.9")}, nil
	}
	ip, err := resolveNFSServer(context.Background(), "nas.lan")
	require.NoError(t, err)
	require.Equal(t, "192.168.1.9", ip.String())
}
