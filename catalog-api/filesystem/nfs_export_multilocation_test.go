package filesystem

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// SEC-DIAL-1 review (CAT-RETEST-STALE-1): Apple's mount_nfs parses a source of the form "host1:/path1,host2:/path2" as several server
// locations, so an export in which a ',' is followed anywhere by ':' would add an unvetted second server to the darwin mount source
// ("<vetted>:/vol,10.0.0.5:/x"), bypassing the operator-class egress policy and re-opening DNS rebinding. Such an export is refused on every
// platform (the validator is shared). A ',' or a space WITHOUT a later ':' stays legal (share names like "/a,b" or "/volume1/My Videos").
func TestValidateNFSExport_RefusesExtraServerLocation(t *testing.T) {
	for _, exp := range []string{
		"/vol,10.0.0.5:/x",
		"/vol,internal.example.com:/x",
		"/vol,[::1]:/x",
		"/vol,:/x",
		"/a,b,c:d",
		"/vol ,host:/x",
		"/vol,/other,10.0.0.5:/x",
	} {
		require.Error(t, ValidateNFSExport(exp), "%q must be refused: a ',' followed by ':' starts another server location", exp)
	}
}

func TestValidateNFSExport_LegalShareNamesStayLegal(t *testing.T) {
	for _, exp := range []string{"/volume1/My Videos", "/export,addr=127.0.0.1", "/a,b", "/vol:1/x", "/a,b/c d", "/x:y,z"} {
		require.NoError(t, ValidateNFSExport(exp), "%q is a legal share name (no ':' after a ',')", exp)
	}
}
