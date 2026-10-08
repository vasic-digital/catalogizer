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
)

var userinfoRE = regexp.MustCompile(`(?i)([a-z][a-z0-9+.\-]*://)[^/\s@"']+@`)

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
			out = append(out, s, url.QueryEscape(s), url.PathEscape(s))
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
	return out
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
