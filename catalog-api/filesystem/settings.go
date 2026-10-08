// settings.go - PA-01 (WP-12): the ONE settings-key vocabulary of the storage protocols, the per-protocol schema, strict
// validation, the protocol registry (pluggable protocols such as sftp / ftps / nfs3), and the single StorageRoot -> settings mapping.
//
// Before this file three copies of the mapping existed (universal scanner, stream handler, comic pages handler) and they
// disagreed with the factory: the NFS export went out as "export_path" while the factory read "path"; the FTP and WebDAV
// "path" was never forwarded. A key the factory does not know is now an ERROR, never a silently ignored value.
//
// Revision: 1 (2026-10-07). Documented in docs/scripts/settings_contract.md.
package filesystem

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"catalogizer/models"
)

// The settings vocabulary. Every key a StorageConfig.Settings map may carry is one of these.
const (
	KeyHost               = "host"
	KeyPort               = "port"
	KeyPath               = "path"
	KeyShare              = "share" // SMB spelling of the root path: the share name
	KeyBasePath           = "base_path"
	KeyURL                = "url"
	KeyUsername           = "username"
	KeyPassword           = "password"
	KeyDomain             = "domain"
	KeyMountPoint         = "mount_point"
	KeyOptions            = "options"
	KeyCredentialRef      = "credential_ref"
	KeyTLSMode            = "tls_mode"
	KeyHostKeyFingerprint = "host_key_fingerprint"
	KeyCertSHA256         = "cert_sha256"
)

// KeyType is the value type of a settings key.
type KeyType int

const (
	// KeyString is a string value.
	KeyString KeyType = iota
	// KeyInt is a whole number (int, int32, int64, or a whole float64 as decoded from JSON).
	KeyInt
)

// ProtocolSpec describes one protocol: the keys its factory consumes and, for a registered (non built-in) protocol, how to build the client.
type ProtocolSpec struct {
	// Name is the protocol name as stored in StorageRoot.Protocol.
	Name string
	// Keys is the closed set of settings keys the protocol consumes.
	Keys map[string]KeyType
	// New builds the client from validated settings. Required for a registered protocol; unused for the built-ins.
	New func(settings map[string]interface{}) (FileSystemClient, error)
}

// legacyKeyHints maps a retired or misspelt key to the vocabulary key to use. Used only to make the rejection message useful.
var legacyKeyHints = map[string]string{
	"export_path": KeyPath,
	"filepath":    KeyPath,
	"file_path":   KeyPath,
	"root":        KeyPath,
	"hostname":    KeyHost,
	"user":        KeyUsername,
	"pass":        KeyPassword,
}

var builtinSpecs = map[string]ProtocolSpec{
	"local":  {Name: "local", Keys: map[string]KeyType{KeyBasePath: KeyString}},
	"smb":    {Name: "smb", Keys: map[string]KeyType{KeyHost: KeyString, KeyPort: KeyInt, KeyShare: KeyString, KeyUsername: KeyString, KeyPassword: KeyString, KeyDomain: KeyString}},
	"ftp":    {Name: "ftp", Keys: map[string]KeyType{KeyHost: KeyString, KeyPort: KeyInt, KeyUsername: KeyString, KeyPassword: KeyString, KeyPath: KeyString}},
	"nfs":    {Name: "nfs", Keys: map[string]KeyType{KeyHost: KeyString, KeyPath: KeyString, KeyMountPoint: KeyString, KeyOptions: KeyString}},
	"webdav": {Name: "webdav", Keys: map[string]KeyType{KeyURL: KeyString, KeyUsername: KeyString, KeyPassword: KeyString, KeyPath: KeyString}},
}

// builtinOrder is the stable order of SupportedProtocols for the built-ins.
var builtinOrder = []string{"smb", "ftp", "nfs", "webdav", "local"}

var (
	registryMu sync.RWMutex
	registered = map[string]ProtocolSpec{}
)

// RegisterProtocol adds a pluggable protocol (sftp, ftps, nfs3, ...) behind the FileSystemClient interface. The name must not be a
// built-in or already registered, and spec.New and spec.Keys must be set. Safe for concurrent use; typically called from an init().
func RegisterProtocol(spec ProtocolSpec) error {
	name := strings.TrimSpace(spec.Name)
	if name == "" || name != spec.Name || strings.ContainsAny(name, " \t/:") {
		return fmt.Errorf("filesystem: invalid protocol name %q", spec.Name)
	}
	if spec.New == nil {
		return fmt.Errorf("filesystem: protocol %q registered without a constructor", name)
	}
	if len(spec.Keys) == 0 {
		return fmt.Errorf("filesystem: protocol %q registered without a settings schema", name)
	}
	if _, ok := builtinSpecs[name]; ok {
		return fmt.Errorf("filesystem: protocol %q is built in and cannot be replaced", name)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registered[name]; ok {
		return fmt.Errorf("filesystem: protocol %q is already registered", name)
	}
	registered[name] = spec
	return nil
}

// UnregisterProtocol removes a registered protocol (never a built-in). It reports whether one was removed. Intended for tests and shutdown.
func UnregisterProtocol(name string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()
	_, ok := registered[name]
	delete(registered, name)
	return ok
}

// IsRegisteredProtocol reports whether name is a registered (non built-in) protocol.
func IsRegisteredProtocol(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registered[name]
	return ok
}

// RegisteredProtocols lists the registered (non built-in) protocol names, sorted.
func RegisteredProtocols() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registered))
	for n := range registered {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func specFor(protocol string) (ProtocolSpec, bool) {
	if s, ok := builtinSpecs[protocol]; ok {
		return s, true
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	s, ok := registered[protocol]
	return s, ok
}

// SettingsError is returned by ValidateSettings. It lists every problem, not just the first.
type SettingsError struct {
	Protocol  string
	Unknown   []string // keys the protocol does not consume, with a hint when the key is a retired spelling
	WrongType []string // keys whose value has the wrong type
}

func (e *SettingsError) Error() string {
	var parts []string
	if len(e.Unknown) > 0 {
		parts = append(parts, "unknown settings: "+strings.Join(e.Unknown, ", "))
	}
	if len(e.WrongType) > 0 {
		parts = append(parts, "wrong type: "+strings.Join(e.WrongType, ", "))
	}
	return fmt.Sprintf("invalid %s settings: %s", e.Protocol, strings.Join(parts, "; "))
}

// ValidateSettings checks settings against the schema of protocol: every key must be consumed by the protocol and have the right type.
// An unsupported protocol is an error. The values are never echoed (they may be secrets), only the key names.
func ValidateSettings(protocol string, settings map[string]interface{}) error {
	spec, ok := specFor(protocol)
	if !ok {
		return fmt.Errorf("unsupported protocol: %s", protocol)
	}
	var unknown, wrong []string
	for k, v := range settings {
		kt, known := spec.Keys[k]
		if !known {
			if hint, ok := legacyKeyHints[k]; ok {
				if _, hintOK := spec.Keys[hint]; hintOK {
					unknown = append(unknown, fmt.Sprintf("%s (use %s)", k, hint))
					continue
				}
			}
			unknown = append(unknown, k)
			continue
		}
		if !typeOK(kt, v) {
			wrong = append(wrong, fmt.Sprintf("%s (want %s)", k, kindName(kt)))
		} else if k == KeyPort {
			if n, _ := wholeNumber(v); n < 1 || n > 65535 {
				wrong = append(wrong, fmt.Sprintf("%s (want a port 1-65535)", k))
			}
		}
	}
	if len(unknown) == 0 && len(wrong) == 0 {
		return nil
	}
	sort.Strings(unknown)
	sort.Strings(wrong)
	return &SettingsError{Protocol: protocol, Unknown: unknown, WrongType: wrong}
}

func kindName(k KeyType) string {
	if k == KeyInt {
		return "integer"
	}
	return "string"
}

func typeOK(k KeyType, v interface{}) bool {
	switch k {
	case KeyString:
		_, ok := v.(string)
		return ok
	case KeyInt:
		_, ok := wholeNumber(v)
		return ok
	}
	return false
}

// wholeNumber reads an integer setting. It accepts int, int32, int64 and a whole float64 (what encoding/json yields), and ONLY values that
// fit a 32-bit signed integer: a port of 1e300 is not a number a client can use, and converting it would give garbage (WF22 F1).
func wholeNumber(v interface{}) (int64, bool) {
	var n int64
	switch x := v.(type) {
	case int:
		n = int64(x)
	case int32:
		return int64(x), true
	case int64:
		n = x
	case float64:
		// checked in the float domain: converting a float outside the int64 range is implementation-defined
		if math.IsNaN(x) || math.IsInf(x, 0) || x != math.Trunc(x) || x < math.MinInt32 || x > math.MaxInt32 {
			return 0, false
		}
		return int64(x), true
	default:
		return 0, false
	}
	// int may be 64 bits wide and int64 always is
	if n < math.MinInt32 || n > math.MaxInt32 {
		return 0, false
	}
	return n, true
}

// SettingsFromRoot is THE mapping from a StorageRoot row to factory settings. smbIdentity resolves the SMB identity (direct fields, then the
// identity_index in Options); it may be nil, in which case only the direct fields are used. Only keys the protocol consumes are produced, so the
// result always passes ValidateSettings. A registered protocol is mapped generically from the root fields its schema allows.
func SettingsFromRoot(root *models.StorageRoot, smbIdentity func(*models.StorageRoot) (user, pass, domain string)) map[string]interface{} {
	s := make(map[string]interface{})
	put := func(k string, v *string) {
		if v != nil {
			s[k] = *v
		}
	}
	putInt := func(k string, v *int) {
		if v != nil && !(k == KeyPort && *v <= 0) { // a stored port of 0 means "not set": the client default applies
			s[k] = *v
		}
	}
	switch root.Protocol {
	case "local":
		put(KeyBasePath, root.Path)
	case "smb":
		put(KeyHost, root.Host)
		putInt(KeyPort, root.Port)
		put(KeyShare, root.Path)
		var user, pass, dom string
		if smbIdentity != nil {
			user, pass, dom = smbIdentity(root)
		} else {
			user, pass, dom = deref(root.Username), deref(root.Password), deref(root.Domain)
		}
		if user != "" {
			s[KeyUsername] = user
		}
		if pass != "" {
			s[KeyPassword] = pass
		}
		if dom != "" {
			s[KeyDomain] = dom
		}
		if root.Domain != nil && *root.Domain != "" { // an empty column must not erase the identity's domain nor the WORKGROUP default (WF22 F4)
			s[KeyDomain] = *root.Domain
		}
	case "ftp":
		put(KeyHost, root.Host)
		putInt(KeyPort, root.Port)
		put(KeyUsername, root.Username)
		put(KeyPassword, root.Password)
		put(KeyPath, root.Path)
	case "nfs":
		put(KeyHost, root.Host)
		put(KeyPath, root.Path)
		put(KeyMountPoint, root.MountPoint)
		put(KeyOptions, root.Options)
	case "webdav":
		put(KeyURL, root.URL)
		put(KeyUsername, root.Username)
		put(KeyPassword, root.Password)
		put(KeyPath, root.Path)
	default:
		spec, ok := specFor(root.Protocol)
		if !ok {
			return s
		}
		allow := func(k string) bool { _, ok := spec.Keys[k]; return ok }
		if allow(KeyHost) {
			put(KeyHost, root.Host)
		}
		if allow(KeyPort) {
			putInt(KeyPort, root.Port)
		}
		if allow(KeyPath) {
			put(KeyPath, root.Path)
		}
		if allow(KeyUsername) {
			put(KeyUsername, root.Username)
		}
		if allow(KeyPassword) {
			put(KeyPassword, root.Password)
		}
		if allow(KeyDomain) {
			put(KeyDomain, root.Domain)
		}
		if allow(KeyURL) {
			put(KeyURL, root.URL)
		}
		if allow(KeyMountPoint) {
			put(KeyMountPoint, root.MountPoint)
		}
		if allow(KeyOptions) {
			put(KeyOptions, root.Options)
		} else if root.Options != nil && *root.Options != "" {
			// Options is a JSON object for non-NFS protocols; it carries the TLS / pin / credential-reference keys.
			var extra map[string]interface{}
			if json.Unmarshal([]byte(*root.Options), &extra) == nil {
				for _, k := range []string{KeyCredentialRef, KeyTLSMode, KeyHostKeyFingerprint, KeyCertSHA256} {
					if v, ok := extra[k].(string); ok && allow(k) && v != "" {
						s[k] = v
					}
				}
			}
		}
	}
	return s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// StringSetting reads a string setting (for protocol constructors registered through RegisterProtocol).
func StringSetting(settings map[string]interface{}, key, def string) string {
	return getStringSetting(settings, key, def)
}

// IntSetting reads an integer setting (for protocol constructors registered through RegisterProtocol).
func IntSetting(settings map[string]interface{}, key string, def int) int {
	return getIntSetting(settings, key, def)
}
