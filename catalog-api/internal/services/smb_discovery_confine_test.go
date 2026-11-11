package services

import (
	"context"
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Unit-class: the in-process memShare (smb_subdir_test.go) stands in for the SMB server only. SplitShare, ResolveSub, confinedShare,
// port validation and BrowseShare/TestConnection are the real code (SMB-FU-2).

func discSvc(m *memShare, opened *[]string) *SMBDiscoveryService {
	s := NewSMBDiscoveryService(zap.NewNop())
	s.openShare = func(_ context.Context, _ SMBConnectionConfig, shareName string) (ShareAPI, func(), error) {
		if opened != nil {
			*opened = append(*opened, shareName)
		}
		return m, func() {}, nil
	}
	return s
}

func discCfg(share string) SMBConnectionConfig {
	return SMBConnectionConfig{Host: "192.0.2.10", Port: 445, Share: share, Username: "u", Password: "p"}
}

func TestSMBDiscovery_BrowseRefusesAdminAndMalformedShares(t *testing.T) {
	for _, bad := range []string{"IPC$", "C$", "ADMIN$", "C$/x", `a\b`, "share/..", "Public/../Misc", "//h/s", "", "Public/CON", "Public/a:b"} {
		m := newMem()
		var opened []string
		_, err := discSvc(m, &opened).BrowseShare(context.Background(), discCfg(bad), ".")
		require.Error(t, err, bad)
		require.True(t, errors.Is(err, ErrInvalidSMBInput), "%q: %v", bad, err)
		require.Empty(t, opened, "%q must never reach a mount", bad)
		require.Empty(t, m.calls, bad)

		opened = nil
		require.False(t, discSvc(m, &opened).TestConnection(context.Background(), discCfg(bad)), bad)
		require.Empty(t, opened, "%q must never reach a mount (TestConnection)", bad)
	}
}

func TestSMBDiscovery_BrowseRefusesParentSegmentsOnPlainShare(t *testing.T) {
	for _, p := range []string{"..", "../..", `..\..`, "docs/../../x"} {
		m := newMem()
		var opened []string
		_, err := discSvc(m, &opened).BrowseShare(context.Background(), discCfg("Public"), p)
		require.Error(t, err, p)
		require.True(t, errors.Is(err, ErrInvalidSMBInput), "%q: %v", p, err)
		require.Empty(t, m.calls, "%q must not reach ReadDir", p)
	}
}

func TestSMBDiscovery_BrowseSubRootIsConfined(t *testing.T) {
	m := newMem()
	var opened []string
	s := discSvc(m, &opened)
	for _, p := range []string{"a:stream", "", ".", "/docs", "../Misc", `..\Misc`, "docs/../../Misc"} {
		_, _ = s.BrowseShare(context.Background(), discCfg("Public/Testing"), p)
	}
	require.NotEmpty(t, m.calls)
	for _, c := range m.calls {
		cc := path.Clean(c)
		require.NotContains(t, c, ":")
		require.True(t, cc == "Testing" || strings.HasPrefix(cc, "Testing/"), "share path %q escaped the sub root (all: %v)", c, m.calls)
	}
	for _, o := range opened {
		require.Equal(t, "Public", o, "only the share part is mounted")
	}
}

func TestSMBDiscovery_BrowseGoodShareWithSubBrowsesWithinSub(t *testing.T) {
	m := newMem()
	var opened []string
	s := discSvc(m, &opened)
	ents, err := s.BrowseShare(context.Background(), discCfg("Public/Testing"), "docs")
	require.NoError(t, err)
	require.Len(t, ents, 1)
	require.Equal(t, "book.txt", ents[0].Name)
	require.Equal(t, "docs/book.txt", ents[0].Path)
	require.Equal(t, []string{"Public"}, opened)
	require.Equal(t, []string{"Testing/docs"}, m.calls)

	m2 := newMem()
	ents, err = discSvc(m2, nil).BrowseShare(context.Background(), discCfg("Public/Testing"), ".")
	require.NoError(t, err)
	require.Len(t, ents, 1)
	require.Equal(t, "docs", ents[0].Name, "root of the sub root, not of the share")
}

func TestSMBDiscovery_PlainShareUnchanged_NegativeControl(t *testing.T) {
	m := newMem()
	var opened []string
	ents, err := discSvc(m, &opened).BrowseShare(context.Background(), discCfg("Public"), "docs")
	require.NoError(t, err)
	require.Len(t, ents, 1)
	require.Equal(t, "book.txt", ents[0].Name)
	require.Equal(t, []string{"Public"}, opened)
	require.Equal(t, []string{"docs"}, m.calls, "a plain share passes the path through")

	m2 := newMem()
	require.True(t, discSvc(m2, nil).TestConnection(context.Background(), discCfg("Public")))
	require.True(t, discSvc(newMem(), nil).TestConnection(context.Background(), discCfg("/Public")), "legacy /share spelling still means the share")
}

func TestSMBDiscovery_TestConnectionConfinedToSub(t *testing.T) {
	m := newMem()
	var opened []string
	require.True(t, discSvc(m, &opened).TestConnection(context.Background(), discCfg("Public/Testing")))
	require.Equal(t, []string{"Public"}, opened)
	require.Equal(t, []string{"Testing"}, m.calls, "ReadDir('.') lists the sub root, not the share root")
}

func TestSMBDiscovery_PortOtherThan445Refused(t *testing.T) {
	require.NoError(t, ValidateSMBPort(445))
	require.NoError(t, ValidateSMBPort(0), "0 means the default 445")
	for _, p := range []int{22, 80, 139, 65535, -1, 70000} {
		require.Error(t, ValidateSMBPort(p), p)
		m := newMem()
		var opened []string
		cfg := discCfg("Public")
		cfg.Port = p
		_, err := discSvc(m, &opened).BrowseShare(context.Background(), cfg, ".")
		require.True(t, errors.Is(err, ErrInvalidSMBInput), "%d: %v", p, err)
		require.False(t, discSvc(m, &opened).TestConnection(context.Background(), cfg))
		require.Empty(t, opened, "port %d must not be dialled", p)
	}
}

func TestSMBDiscovery_MountErrorIsGeneric(t *testing.T) {
	s := NewSMBDiscoveryService(zap.NewNop())
	s.openShare = func(context.Context, SMBConnectionConfig, string) (ShareAPI, func(), error) {
		return nil, nil, errors.New("dial tcp 10.9.8.7:445: STATUS_LOGON_FAILURE internal-detail")
	}
	_, err := s.BrowseShare(context.Background(), discCfg("Public"), ".")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "10.9.8.7")
	require.NotContains(t, err.Error(), "internal-detail")
	require.NotContains(t, err.Error(), "STATUS_")
	require.False(t, errors.Is(err, ErrInvalidSMBInput), "a mount failure is not a client input error")
}

// SMB-FU-10: ADS and Windows device names are refused on plain-share browse paths before any mount.
func TestSMBDiscovery_BrowseRefusesADSAndDeviceNamePaths(t *testing.T) {
	for _, bad := range []string{"a:b", "f.txt:stream", "CON", "dir/nul.txt", `dir\COM1`, "LPT9"} {
		m := newMem()
		var opened []string
		_, err := discSvc(m, &opened).BrowseShare(context.Background(), discCfg("Public"), bad)
		require.Error(t, err, bad)
		require.True(t, errors.Is(err, ErrInvalidSMBInput), "%q: %v", bad, err)
		require.Empty(t, opened, "%q must never reach a mount", bad)
	}
}
