package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WF22 round 3, class B (N5): no serialisation of a StorageRoot returns a credential. The classifier enumerates EVERY field of the struct, so a
// column added later must be classified here (exposed or redacted) before the suite passes.
func TestWF22R3_StorageRootMarshalNeverCarriesACredential(t *testing.T) {
	// field name -> how it appears in JSON: "exposed" verbatim, "redacted" (the password of the url's userinfo / the column is absent)
	classification := map[string]string{
		"ID": "exposed", "Name": "exposed", "Protocol": "exposed", "Host": "exposed", "Port": "exposed", "Path": "exposed", "Username": "exposed",
		"Password": "absent", "Domain": "exposed", "MountPoint": "exposed", "Options": "exposed", "URL": "redacted", "Enabled": "exposed",
		"MaxDepth": "exposed", "AllowEmpty": "exposed", "EnableDuplicateDetection": "exposed", "EnableMetadataExtraction": "exposed",
		"IncludePatterns": "exposed", "ExcludePatterns": "exposed", "CreatedAt": "exposed", "UpdatedAt": "exposed", "LastScanAt": "exposed",
	}
	rt := reflect.TypeOf(StorageRoot{})
	for i := 0; i < rt.NumField(); i++ {
		_, ok := classification[rt.Field(i).Name]
		require.True(t, ok, "StorageRoot.%s is not classified: decide whether it may appear in an API response", rt.Field(i).Name)
	}
	require.Equal(t, len(classification), rt.NumField())

	pw, url, user := "colSecret88", "https://alice:urlSecret77@nas/dav", "bob"
	r := StorageRoot{ID: 7, Name: "dav", Protocol: "webdav", Username: &user, Password: &pw, URL: &url}
	for name, v := range map[string]interface{}{"value": r, "pointer": &r, "slice": []StorageRoot{r}, "slice of pointers": []*StorageRoot{&r}} {
		b, err := json.Marshal(v)
		require.NoError(t, err, name)
		s := string(b)
		assert.NotContains(t, s, "colSecret88", name)
		assert.NotContains(t, s, "urlSecret77", name)
		assert.Contains(t, s, `"username":"bob"`, "%s: control: the non-secret columns are there", name)
		assert.Contains(t, s, "alice:xxxxx@nas", "%s: the url keeps its host and user, the password is masked", name)
	}
	// an unparsable url is not echoed
	bad := "http://u:secret@%zz"
	b, err := json.Marshal(StorageRoot{URL: &bad})
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(b), "secret"))
	// the in-memory row is untouched (the seam redacts a copy)
	assert.Equal(t, "colSecret88", *r.Password)
	assert.Equal(t, "https://alice:urlSecret77@nas/dav", *r.URL)
}
