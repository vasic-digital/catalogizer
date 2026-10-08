package filesystem

import (
	"errors"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PA-01: the strict settings contract of the client factory (unit tests; the in-memory fake client below is unit-only).

type fakeRegisteredClient struct {
	FileSystemClient
	settings map[string]interface{}
}

func spec(name string, newFn func(map[string]interface{}) (FileSystemClient, error)) ProtocolSpec {
	return ProtocolSpec{
		Name: name,
		Keys: map[string]KeyType{KeyHost: KeyString, KeyPort: KeyInt, KeyPath: KeyString, KeyUsername: KeyString, KeyCredentialRef: KeyString, KeyTLSMode: KeyString, KeyHostKeyFingerprint: KeyString},
		New:  newFn,
	}
}

func TestValidateSettings_RejectsUnknownKeyWithHint(t *testing.T) {
	err := ValidateSettings("nfs", map[string]interface{}{"host": "h", "export_path": "/e"})
	require.Error(t, err)
	var se *SettingsError
	require.True(t, errors.As(err, &se))
	assert.Equal(t, []string{"export_path (use path)"}, se.Unknown)
	assert.Contains(t, err.Error(), "export_path")
	assert.NotContains(t, err.Error(), "/e", "values must never be echoed: they can be secrets")
}

func TestValidateSettings_RejectsEveryUnknownKeyNotJustTheFirst(t *testing.T) {
	err := ValidateSettings("ftp", map[string]interface{}{"host": "h", "zzz": "1", "aaa": "2", "filepath": "/x"})
	var se *SettingsError
	require.True(t, errors.As(err, &se))
	assert.Equal(t, []string{"aaa", "filepath (use path)", "zzz"}, se.Unknown)
}

func TestValidateSettings_WrongTypes(t *testing.T) {
	err := ValidateSettings("ftp", map[string]interface{}{"host": 5, "port": "21", "username": "u"})
	var se *SettingsError
	require.True(t, errors.As(err, &se))
	assert.Equal(t, []string{"host (want string)", "port (want integer)"}, se.WrongType)
	assert.Empty(t, se.Unknown)
}

func TestValidateSettings_IntegerSpellings(t *testing.T) {
	for _, v := range []interface{}{21, int32(21), int64(21), float64(21)} {
		assert.NoError(t, ValidateSettings("ftp", map[string]interface{}{"port": v}), "%T", v)
	}
	assert.Error(t, ValidateSettings("ftp", map[string]interface{}{"port": 21.5}), "a fractional JSON number is not a port")
}

func TestValidateSettings_UnsupportedProtocol(t *testing.T) {
	err := ValidateSettings("gopher", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported protocol: gopher")
}

func TestValidateSettings_NilAndEmptyAreValid(t *testing.T) {
	for _, p := range []string{"local", "smb", "ftp", "nfs", "webdav"} {
		assert.NoError(t, ValidateSettings(p, nil), p)
		assert.NoError(t, ValidateSettings(p, map[string]interface{}{}), p)
	}
}

func TestFactory_CreateClientRejectsUnknownKey(t *testing.T) {
	f := NewDefaultClientFactory()
	_, err := f.CreateClient(&StorageConfig{Protocol: "nfs", Settings: map[string]interface{}{"host": "h", "export_path": "/e", "mount_point": "/mnt/x"}})
	require.Error(t, err, "the old scanner spelling must now fail loudly instead of producing an empty export")
	assert.Contains(t, err.Error(), "export_path")
}

func TestFactory_CreateClientAcceptsEveryBuiltinKey(t *testing.T) {
	f := NewDefaultClientFactory()
	full := map[string]map[string]interface{}{
		"local":  {"base_path": "/tmp"},
		"smb":    {"host": "h", "port": 445, "share": "s", "username": "u", "password": "p", "domain": "d"},
		"ftp":    {"host": "h", "port": 21, "username": "u", "password": "p", "path": "/p"},
		"nfs":    {"host": "h", "path": "/e", "mount_point": "/mnt/x", "options": "vers=3"},
		"webdav": {"url": "http://h/", "username": "u", "password": "p", "path": "/p"},
	}
	for proto, settings := range full {
		c, err := f.CreateClient(&StorageConfig{Protocol: proto, Settings: settings})
		require.NoError(t, err, proto)
		require.NotNil(t, c, proto)
	}
}

func TestRegisterProtocol_Lifecycle(t *testing.T) {
	name := "testproto-lifecycle"
	t.Cleanup(func() { UnregisterProtocol(name) })
	var got map[string]interface{}
	require.NoError(t, RegisterProtocol(spec(name, func(s map[string]interface{}) (FileSystemClient, error) {
		got = s
		return &fakeRegisteredClient{settings: s}, nil
	})))

	assert.True(t, IsRegisteredProtocol(name))
	assert.Contains(t, RegisteredProtocols(), name)
	f := NewDefaultClientFactory()
	assert.Contains(t, f.SupportedProtocols(), name)
	assert.Equal(t, []string{"smb", "ftp", "nfs", "webdav", "local"}, f.SupportedProtocols()[:5], "built-ins keep their order and come first")

	c, err := f.CreateClient(&StorageConfig{Protocol: name, Settings: map[string]interface{}{"host": "h", "path": "/p", "credential_ref": "ref1"}})
	require.NoError(t, err)
	assert.Equal(t, "ref1", StringSetting(c.(*fakeRegisteredClient).settings, "credential_ref", ""))
	assert.Equal(t, "h", StringSetting(got, "host", ""))

	// a registered protocol is validated against ITS schema too
	_, err = f.CreateClient(&StorageConfig{Protocol: name, Settings: map[string]interface{}{"password": "inline-secret"}})
	require.Error(t, err, "a key outside the registered schema (inline password) is rejected before the constructor runs")
	assert.NotContains(t, err.Error(), "inline-secret")

	assert.True(t, UnregisterProtocol(name))
	_, err = f.CreateClient(&StorageConfig{Protocol: name, Settings: map[string]interface{}{"host": "h"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported protocol")
}

func TestRegisterProtocol_Refusals(t *testing.T) {
	ok := func(map[string]interface{}) (FileSystemClient, error) { return nil, nil }
	assert.Error(t, RegisterProtocol(spec("smb", ok)), "a built-in cannot be replaced")
	assert.Error(t, RegisterProtocol(spec("", ok)))
	assert.Error(t, RegisterProtocol(spec("has space", ok)))
	assert.Error(t, RegisterProtocol(spec("x/y", ok)))
	assert.Error(t, RegisterProtocol(ProtocolSpec{Name: "noctor", Keys: map[string]KeyType{KeyHost: KeyString}}), "constructor required")
	assert.Error(t, RegisterProtocol(ProtocolSpec{Name: "noschema", New: ok}), "schema required")
	dup := "testproto-dup"
	t.Cleanup(func() { UnregisterProtocol(dup) })
	require.NoError(t, RegisterProtocol(spec(dup, ok)))
	assert.Error(t, RegisterProtocol(spec(dup, ok)), "double registration")
	assert.False(t, UnregisterProtocol("smb"), "a built-in is never unregistered")
}

func TestSettingsFromRoot_RegisteredProtocolUsesItsSchemaOnly(t *testing.T) {
	name := "testproto-fromroot"
	t.Cleanup(func() { UnregisterProtocol(name) })
	require.NoError(t, RegisterProtocol(spec(name, func(map[string]interface{}) (FileSystemClient, error) { return nil, nil })))
	host, user, pass, path := "h", "u", "inline-secret", "/media"
	port := 2222
	opts := `{"credential_ref":"nas-1","tls_mode":"explicit","host_key_fingerprint":"SHA256:abc","cert_sha256":"not-in-schema","junk":"x"}`
	s := SettingsFromRoot(&models.StorageRoot{Protocol: name, Host: &host, Port: &port, Path: &path, Username: &user, Password: &pass, Options: &opts}, nil)
	assert.Equal(t, map[string]interface{}{"host": "h", "port": 2222, "path": "/media", "username": "u", "credential_ref": "nas-1", "tls_mode": "explicit", "host_key_fingerprint": "SHA256:abc"}, s,
		"password is not in the schema so it is never forwarded; cert_sha256 is not in this schema; junk is dropped")
	require.NoError(t, ValidateSettings(name, s))
}

func TestSettingsFromRoot_UnknownProtocolIsEmpty(t *testing.T) {
	h := "h"
	assert.Empty(t, SettingsFromRoot(&models.StorageRoot{Protocol: "gopher", Host: &h}, nil))
}

func TestSettingsFromRoot_AlwaysValidatesForBuiltins(t *testing.T) {
	str := func(s string) *string { return &s }
	port := 1
	for _, proto := range []string{"local", "smb", "ftp", "nfs", "webdav"} {
		r := &models.StorageRoot{Protocol: proto, Host: str("h"), Port: &port, Path: str("/p"), Username: str("u"), Password: str("p"), Domain: str("d"), MountPoint: str("/m"), Options: str("o"), URL: str("http://h/")}
		assert.NoError(t, ValidateSettings(proto, SettingsFromRoot(r, nil)), proto)
	}
}

func TestSettingsFromRoot_SMBIdentityResolverWins(t *testing.T) {
	r := &models.StorageRoot{Protocol: "smb"}
	s := SettingsFromRoot(r, func(*models.StorageRoot) (string, string, string) { return "resolved-user", "resolved-pass", "RD" })
	assert.Equal(t, "resolved-user", s["username"])
	assert.Equal(t, "resolved-pass", s["password"])
	assert.Equal(t, "RD", s["domain"])
}
