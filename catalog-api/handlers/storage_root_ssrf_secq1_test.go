package handlers

import (
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

func TestSECQ1_StorageRootURLEarlyCheck(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	s := func(v string) *string { return &v }
	for _, u := range []string{"http://127.0.0.1:5005/", "http://169.254.169.254/", "https://[fe80::1]/dav", "ftp://192.168.1.2/", "http://user:pw@169.254.169.254/"} {
		require.Error(t, validateWebDAVRootURL(s(u)), u)
	}
	for _, u := range []string{"http://192.168.1.20:5005/", "https://webdav.yandex.ru/", "http://user:pw@192.168.1.20/dav"} {
		require.NoError(t, validateWebDAVRootURL(s(u)), u)
	}
	require.Error(t, validateWebDAVRootURL(nil))
}
