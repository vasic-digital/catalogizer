package filesystem

// WF22 independent-review probes (scratch copy only). Polarity: assert the correct behaviour; FAIL = defect present.

import (
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F1: a whole float far outside the int range passes the KeyInt type check and IntSetting converts it to garbage.
func TestReviewProbe_F1_IntOverflowPassesValidation(t *testing.T) {
	require.Error(t, ValidateSettings("ftp", map[string]interface{}{"port": 21.5}), "control: a fractional port is rejected")
	err := ValidateSettings("ftp", map[string]interface{}{"port": 1e300})
	t.Logf("F1 port=1e300 -> err=%v IntSetting=%d", err, IntSetting(map[string]interface{}{"port": 1e300}, "port", 21))
	assert.Error(t, err, "a port of 1e300 is not an integer the client can use")
}

// F2: the PROPFIND self entry is compared case-sensitively and byte-exact: a server that canonicalises the collection href (IIS / case
// insensitive, or the configured URL spelled differently) gets the collection itself listed as its own child.
func TestReviewProbe_F2_PropfindSelfMismatchListsSelfAsChild(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">` +
		`<D:response><D:href>/dav/movies/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>` +
		`<D:response><D:href>/dav/movies/a.mkv</D:href><D:propstat><D:prop><D:resourcetype/></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>` +
		`</D:multistatus>`)
	same, err := parsePropfind(body, "http://nas/dav/movies/")
	require.NoError(t, err)
	require.Len(t, same, 1, "control: with the exact spelling the self entry is skipped")
	out, err := parsePropfind(body, "http://nas/DAV/Movies/")
	require.NoError(t, err)
	var names []string
	for _, f := range out {
		names = append(names, f.Name)
	}
	t.Logf("F2 request /DAV/Movies/ -> children %v", names)
	assert.Len(t, out, 1, "the collection itself must not come back as its own child")
}

// F3: an href outside the requested collection is accepted as a child of it.
func TestReviewProbe_F3_PropfindForeignHrefAccepted(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">` +
		`<D:response><D:href>/dav/movies/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>/dav/movies/a.mkv</D:href><D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>/etc/passwd</D:href><D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response>` +
		`</D:multistatus>`)
	out, err := parsePropfind(body, "http://nas/dav/movies/")
	require.NoError(t, err)
	var names []string
	for _, f := range out {
		names = append(names, f.Name)
	}
	t.Logf("F3 children %v", names)
	require.Contains(t, names, "a.mkv", "control")
	assert.NotContains(t, names, "passwd", "an href outside the listed collection is not one of its children")
}

// F4: an SMB root whose Domain column is an empty string (not NULL) overrides the identity resolver's domain with "" (and the factory then
// does not apply its WORKGROUP default). Pre-existing behaviour carried into the single mapping.
func TestReviewProbe_F4_EmptyDomainColumnOverridesIdentity(t *testing.T) {
	h, empty := "nas", ""
	root := &models.StorageRoot{Protocol: "smb", Host: &h, Domain: &empty}
	s := SettingsFromRoot(root, func(*models.StorageRoot) (string, string, string) { return "u", "p", "CORP" })
	require.Equal(t, "u", s[KeyUsername], "control: the resolver is used")
	t.Logf("F4 domain=%q", s[KeyDomain])
	assert.Equal(t, "CORP", s[KeyDomain], "an empty domain column should not erase the identity's domain")
}
