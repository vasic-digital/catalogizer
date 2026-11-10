package filesystem

import (
	"context"
	"os"
	"strings"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

func smbRootWithPath(p string) *models.StorageRoot {
	h, u, pw := "10.0.0.5", "u", "p"
	return &models.StorageRoot{Protocol: "smb", Host: &h, Path: &p, Username: &u, Password: &pw}
}

func TestSMBSubdir_ValidateRoot(t *testing.T) {
	for _, p := range []string{"Public", "Public/Testing", "Public/a/b", "Public/Testing/"} {
		require.NoError(t, ValidateRoot(smbRootWithPath(p)), p)
	}
	for _, p := range []string{"/Public/Testing", "Public/../x", "Public/..", `Public\x`, "Public//x", "Public/./x", "Public/x\x00"} {
		require.Error(t, ValidateRoot(smbRootWithPath(p)), p)
	}
}

func TestSMBSubdir_ClientConfinement(t *testing.T) {
	c := NewSmbClient(&SmbConfig{Host: "h", Port: 445, Share: "Public/Testing"})
	name, err := c.initShare()
	require.NoError(t, err)
	require.Equal(t, "Public", name)
	require.Equal(t, "Testing/a/b.txt", c.resolvePath("a/b.txt"))
	require.Equal(t, "Testing", c.resolvePath(""))
	require.Equal(t, "Testing", c.resolvePath("."))
	require.Equal(t, "Testing/etc/passwd", c.resolvePath("../../etc/passwd"))
	require.Equal(t, "Testing/x", c.resolvePath(`..\..\x`))
	require.Equal(t, "Testing/x", c.resolvePath("/x"))

	plain := NewSmbClient(&SmbConfig{Host: "h", Port: 445, Share: "Public"})
	_, err = plain.initShare()
	require.NoError(t, err)
	require.Equal(t, "movies/a", plain.resolvePath("movies/a"))
	require.Equal(t, "", plain.resolvePath(""))

	// F4: the sub-directory is fixed at Connect; changing the config afterwards cannot widen it, and an invalid spec never leaves a usable client
	c.config.Share = "Public/../x"
	require.Equal(t, "Testing/a", c.resolvePath("a"))
	// an invalid spec fails Connect before any network traffic
	bad := NewSmbClient(&SmbConfig{Host: "h", Port: 445, Share: "Public/../x"})
	require.Error(t, bad.Connect(context.Background()))
}

// F3/F2: the smb package client (download / copy) is under the same confinement classifier as the filesystem clients.
func TestSMBSubdir_PackageClientEveryPathGoesThroughRP(t *testing.T) {
	b, err := os.ReadFile("../smb/client.go")
	require.NoError(t, err)
	require.Empty(t, unconfinedPathMethods(t, "client.go", string(b), map[string]bool{"SmbClient": true}))
	// control needle: the classifier flags a method that hands the raw path to the share
	bad := strings.Replace(string(b), "c.share.Remove(c.rp(path))", "c.share.Remove(path)", 1)
	require.NotEqual(t, string(b), bad)
	require.Equal(t, []string{"SmbClient.DeleteFile(path)"}, unconfinedPathMethods(t, "client.go", bad, map[string]bool{"SmbClient": true}))
	bad2 := strings.Replace(string(b), "c.share.Create(c.rp(dstPath))", "c.share.Create(dstPath)", 1)
	require.NotEqual(t, string(b), bad2)
	require.Equal(t, []string{"SmbClient.CopyFile(dstPath)"}, unconfinedPathMethods(t, "client.go", bad2, map[string]bool{"SmbClient": true}))
}

// F6: legacy UNC-form roots are refused with a hint (not silently re-interpreted).
func TestSMBSubdir_UNCRootRefusedWithHint(t *testing.T) {
	for _, p := range []string{`\\h\Public`, "//h/Public"} {
		err := ValidateRoot(smbRootWithPath(p))
		require.Error(t, err, p)
		require.Contains(t, err.Error(), "UNC", p)
	}
}
