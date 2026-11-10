package smb

import (
	"fmt"
	pathpkg "path"
	"strings"
)

// SplitShare splits a storage-root path of the form "share" or "share/sub/dir" into the SMB share name (what is mounted) and the
// sub-directory inside it that the root is confined to ("" for a plain share name, so existing roots behave exactly as before).
//
// The spec is validated strictly because it is stored once and trusted afterwards: it must be relative (a lone leading "/" before the share name is tolerated for old roots), must not
// contain a backslash, a control character or NUL, and must not contain an empty, "." or ".." segment. A single trailing "/" is ignored.
func SplitShare(spec string) (share, sub string, err error) {
	if spec == "" {
		return "", "", fmt.Errorf("smb path is empty: want a share name or share/sub/dir")
	}
	for _, r := range spec {
		if r == '\\' {
			return "", "", fmt.Errorf("smb path %q: backslash is not allowed, use / between segments", spec)
		}
		if r < 0x20 || r == 0x7f {
			return "", "", fmt.Errorf("smb path: control character is not allowed")
		}
	}
	trimmed := strings.TrimSuffix(spec, "/")
	// Legacy spelling: "/share" (a single segment) was accepted before sub-directories existed and still means the share. An absolute path
	// with a sub-directory ("/share/sub") is ambiguous and refused.
	if strings.HasPrefix(trimmed, "/") && !strings.Contains(trimmed[1:], "/") {
		trimmed = trimmed[1:]
	} else if strings.HasPrefix(trimmed, "/") {
		return "", "", fmt.Errorf("smb path %q: must be relative (share or share/sub/dir)", spec)
	}
	segs := strings.Split(trimmed, "/")
	for _, s := range segs {
		if s == "" || s == "." || s == ".." {
			return "", "", fmt.Errorf("smb path %q: empty, \".\" and \"..\" segments are not allowed", spec)
		}
	}
	return segs[0], strings.Join(segs[1:], "/"), nil
}

// ResolveSub maps a root-relative path p to the share-relative path the server is asked for. With no sub root the path is returned
// unchanged (backwards compatible). With a sub root, p is first confined (backslash is a separator on the wire, so it is treated as one;
// ".." can never climb above the root, a leading "/" is read as root-relative) and then joined below sub, so the result is always sub or
// below it. The confinement is lexical: it cannot see a symbolic link the server itself resolves.
func ResolveSub(sub, p string) string {
	if sub == "" {
		return p
	}
	p = strings.ReplaceAll(p, `\`, "/")
	return pathpkg.Join(sub, pathpkg.Clean("/"+p))
}
