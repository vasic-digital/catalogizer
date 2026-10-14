// confine.go - WF22 round 3 (structural fix, class A "path containment"): the ONE path-confinement rule every protocol client uses.
//
// Before this file each client carried its own resolvePath: FTP did path.Join(base, p) (so "/../../etc" under "/srv/movies" became "/etc"),
// NFS and local did strings.ReplaceAll(clean, "..", "") (so the NAME "Wait.. Live" became "Wait Live" and its subtree silently vanished), and only
// WebDAV had it right. The rule is now written once:
//
//   - a path is a slash path whatever the host OS;
//   - it is cleaned AS IF ROOTED, so a ".." SEGMENT can never climb above the root it is joined to;
//   - a NAME that merely CONTAINS ".." ("Wait.. Live", "...And Justice for All") is a name and is kept as it is.
package filesystem

import (
	pathpkg "path"
	"strings"
)

// ConfinePath returns p as a cleaned, rooted slash path ("/a/b"; "" and "." become "/"). A ".." segment cannot leave the root. A remote path is
// always a slash path (package path, never path/filepath - WF22 X1); a caller holding an OS path converts it with filepath.ToSlash first.
func ConfinePath(p string) string {
	return pathpkg.Clean("/" + p)
}

// JoinConfined joins base and p so the result is always base or below it: base + ConfinePath(p). base is used as given (it is the trusted,
// configured root); the result is a slash path.
func JoinConfined(base, p string) string {
	return pathpkg.Join(base, ConfinePath(p))
}

// hasDotDotSegment reports whether p has a ".." path SEGMENT (not a name that merely contains two dots).
func hasDotDotSegment(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// ConfineKeepForm confines p (ConfinePath) and keeps the form the caller used: "" stays "" (the root), a relative path stays relative, a rooted
// path stays rooted, "." stays ".".
func ConfineKeepForm(p string) string {
	if p == "" {
		return ""
	}
	c := ConfinePath(p)
	if strings.HasPrefix(p, "/") {
		return c
	}
	if rel := strings.TrimPrefix(c, "/"); rel != "" || p != "." {
		return rel
	}
	return "."
}
