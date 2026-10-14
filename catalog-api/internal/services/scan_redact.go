// scan_redact.go - WF22 fix round 2 (R9): a scan "reason" is returned by the API and published on the event bus, so it must never carry a credential.
//
// Two layers, because a client error can embed a URL (userinfo) or an echoed server answer: the clients redact what they know they are putting
// into an error (filesystem.redactURL), and this file scrubs whatever still reaches the Reason seam: URL userinfo in any "scheme://user:pass@host"
// text, and the literal secrets of the storage root being scanned (its password, a URL password), plain and URL-escaped.
package services

import (
	"net/url"
	"regexp"
	"strings"

	"catalogizer/models"

	"go.uber.org/zap"
)

// userinfoRE matches "scheme://userinfo@" where userinfo may itself contain "@" (a password such as "p@ssw0rd"): the userinfo runs to the LAST "@"
// of the authority, which ends at the first "/", whitespace or quote. A leftmost-longest match over [^/\s"']* does exactly that (WF22 N9).
var userinfoRE = regexp.MustCompile(`(?i)([a-z][a-z0-9+.\-]*://)[^/\s"']*@`)

// minSecretLen is the shortest literal secret that is scrubbed by text replacement. A shorter one would also destroy ordinary words in the message.
const minSecretLen = 3

// secretsOf lists the literal secrets of a storage root.
func secretsOf(root *models.StorageRoot) []string {
	if root == nil {
		return nil
	}
	var out []string
	add := func(s string) {
		if len(s) >= minSecretLen {
			// the spellings a secret takes in an error: raw, query-escaped, path-escaped, and as the password of a URL's userinfo (url.URL.String)
			out = append(out, s, url.QueryEscape(s), url.PathEscape(s), strings.TrimPrefix(url.UserPassword("", s).String(), ":"))
		}
	}
	if root.Password != nil {
		add(*root.Password)
	}
	if root.URL != nil {
		if u, err := url.Parse(*root.URL); err == nil && u.User != nil {
			if pw, ok := u.User.Password(); ok {
				add(pw)
			}
		}
	}
	// The SMB identity password is resolved from the environment (identity_index) at connect time and never stored in the row: it must be
	// scrubbed too, or an error that echoes it reaches the reason and the logs (WF22 N9).
	if _, pw, _ := ResolveSMBIdentity(root); pw != "" {
		add(pw)
	}
	return out
}

// errField is THE log field for an error that may carry a client / server answer: the text is scrubbed with the same secrets as the scan
// Reason, so a log line can never say what the Reason would not (WF22 N10). Use it instead of zap.Error wherever a filesystem error is logged.
func errField(err error, secrets []string) zap.Field {
	if err == nil {
		return zap.Skip()
	}
	return zap.String("error", scrubSecrets(err.Error(), secrets))
}

// scrubSecrets removes URL userinfo and the given literal secrets from msg.
func scrubSecrets(msg string, secrets []string) string {
	msg = userinfoRE.ReplaceAllString(msg, "${1}***@")
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, "***")
		}
	}
	return msg
}

// ScrubErrorFor returns err's text with the credentials of root removed (URL userinfo, the root's password, its URL password, the env-resolved
// SMB identity password): the one call a handler uses to put a client error into a log line (WF22 N10, class B).
func ScrubErrorFor(err error, root *models.StorageRoot) string {
	if err == nil {
		return ""
	}
	return scrubSecrets(err.Error(), secretsOf(root))
}
