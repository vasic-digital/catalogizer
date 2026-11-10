package smb

import (
	"fmt"
	pathpkg "path"
	"regexp"
	"strings"
)

var (
	driveShare = regexp.MustCompile(`^[A-Za-z]\$$`)
	// Windows device names: reserved with or without an extension (CON, CON.txt) on SMB servers that front a Windows file system.
	deviceName = regexp.MustCompile(`^(?i:CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])$`)
)

// badRune reports a character that must never appear in a root path: ASCII and C1 controls, bidi controls (display spoofing).
func badRune(r rune) bool {
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

// SplitShare splits a storage-root path of the form "share" or "share/sub/dir" into the SMB share name (what is mounted) and the
// sub-directory inside it that the root is confined to ("" for a plain share name, so existing roots behave exactly as before).
//
// The spec is validated strictly because it is stored once and trusted afterwards. It must be relative (a lone legacy "/share" is
// tolerated), must not contain a backslash, control, C1 or bidi-control character, and no segment may be empty, ".", ".." or reduce to
// one of those once trailing dots and spaces are removed ("...", ".. "; Windows servers strip them). The share may not be an
// administrative share (IPC$, ADMIN$, a drive share such as C$). Sub-directory segments may not be Windows device names (CON, NUL,
// COM1..9, LPT1..9, with or without an extension) and may not contain ":" (alternate data streams). One trailing "/" is ignored.
// UNC spellings (\\host\share, //host/share) are refused with a hint: host and share belong in separate fields.
func SplitShare(spec string) (share, sub string, err error) {
	if spec == "" {
		return "", "", fmt.Errorf("smb path is empty: want a share name or share/sub/dir")
	}
	if strings.HasPrefix(spec, `\\`) || strings.HasPrefix(spec, "//") {
		return "", "", fmt.Errorf("smb path %q: UNC paths are not supported; put the server in 'host' and 'share' or 'share/sub/dir' in 'path'", spec)
	}
	for _, r := range spec {
		if r == '\\' {
			return "", "", fmt.Errorf("smb path %q: backslash is not allowed, use / between segments", spec)
		}
		if badRune(r) {
			return "", "", fmt.Errorf("smb path: control or bidi-control character is not allowed")
		}
	}
	trimmed := strings.TrimSuffix(spec, "/")
	// Legacy spelling: "/share" (a single segment) was accepted before sub-directories existed and still means the share. An absolute path
	// with a sub-directory ("/share/sub") is ambiguous and refused.
	if strings.HasPrefix(trimmed, "/") && !strings.Contains(trimmed[1:], "/") {
		trimmed = trimmed[1:]
	} else if strings.HasPrefix(trimmed, "/") {
		return "", "", fmt.Errorf("smb path %q: must be relative (share or share/sub/dir); '/share' is tolerated, '/share/sub' is not", spec)
	}
	segs := strings.Split(trimmed, "/")
	for i, s := range segs {
		if strings.TrimRight(s, ". ") == "" {
			return "", "", fmt.Errorf("smb path %q: empty, \".\" and \"..\" segments (also with trailing dots or spaces) are not allowed", spec)
		}
		if i == 0 {
			if up := strings.ToUpper(s); up == "IPC$" || up == "ADMIN$" || driveShare.MatchString(s) {
				return "", "", fmt.Errorf("smb path %q: administrative share %q is not allowed", spec, s)
			}
			continue
		}
		if strings.Contains(s, ":") {
			return "", "", fmt.Errorf("smb path %q: ':' (alternate data stream) is not allowed in a sub-directory name", spec)
		}
		base := strings.TrimRight(s, ". ")
		if dot := strings.Index(base, "."); dot >= 0 {
			base = base[:dot]
		}
		if deviceName.MatchString(strings.TrimRight(base, " ")) {
			return "", "", fmt.Errorf("smb path %q: reserved device name %q is not allowed in a sub-directory name", spec, s)
		}
	}
	return segs[0], strings.Join(segs[1:], "/"), nil
}

// ResolveSub maps a root-relative path p to the share-relative path the server is asked for. With no sub root the path is returned
// unchanged (backwards compatible). With a sub root, p is first confined (backslash is a separator on the wire, so it is treated as one;
// a segment that is ".." once trailing dots and spaces are removed ("...", ".. ") counts as ".."; ".." can never climb above the root;
// a leading "/" is read as root-relative) and then joined below sub, so the result is always sub or below it. The confinement is
// lexical: it cannot see a symbolic link the server itself resolves.
func ResolveSub(sub, p string) string {
	if sub == "" {
		return p
	}
	segs := strings.Split(strings.ReplaceAll(p, `\`, "/"), "/")
	for i, s := range segs {
		if s != "" && s != "." && strings.TrimRight(s, ". ") == "" {
			segs[i] = ".."
		}
	}
	return pathpkg.Join(sub, pathpkg.Clean("/"+strings.Join(segs, "/")))
}
