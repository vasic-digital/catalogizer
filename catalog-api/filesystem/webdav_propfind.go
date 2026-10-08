package filesystem

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// webdav_propfind.go - PA-03 (WP-12): a namespace-aware PROPFIND multistatus parser.
//
// The previous parser split the body on the literal text "<D:response>", so it found NOTHING in the answer of a real Apache mod_dav server
// (`<D:response xmlns:lp1="DAV:" ...>`, `<lp1:resourcetype>`), and a WebDAV scan of the repository's own test fixture listed an empty root. The
// tags below carry no namespace, so they match elements of any namespace and any prefix (D:, d:, lp1:, ns0:, none).

type davMultistatus struct {
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href      string        `xml:"href"`
	Propstats []davPropstat `xml:"propstat"`
}

type davPropstat struct {
	Status string  `xml:"status"`
	Prop   davProp `xml:"prop"`
}

// davProp carries the properties the catalog uses. DAV:displayname is deliberately NOT read: it is a presentation string ("My Documents" for
// /Docs/), not the identity of the resource; the name of an entry is the last segment of its href (WF22 R10).
type davProp struct {
	ContentLength string          `xml:"getcontentlength"`
	LastModified  string          `xml:"getlastmodified"`
	ETag          string          `xml:"getetag"`
	ResourceType  davResourceType `xml:"resourcetype"`
}

type davResourceType struct {
	Collection *struct{} `xml:"collection"`
	Directory  *struct{} `xml:"directory"`
}

// davStatusOK reports whether a propstat status line is a 2xx; a propstat without a status line is taken as OK.
func davStatusOK(status string) bool {
	status = strings.TrimSpace(status)
	if status == "" {
		return true
	}
	f := strings.Fields(status)
	if len(f) < 2 {
		return false
	}
	code, err := strconv.Atoi(f[1])
	return err == nil && code >= 200 && code < 300
}

// parseDAVTime parses getlastmodified; a missing or unparsable value is the zero time (never "now": the value feeds change detection).
func parseDAVTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, "Mon, 2 Jan 2006 15:04:05 MST", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// hrefPath returns the decoded, slash-cleaned path of an href that may be absolute (http://host/p) or path-only (/p).
func hrefPath(href string) string {
	href = strings.TrimSpace(href)
	if u, err := url.Parse(href); err == nil {
		p := u.Path
		if p == "" {
			p = href
		}
		return strings.TrimRight(p, "/")
	}
	if p, err := url.PathUnescape(href); err == nil {
		return strings.TrimRight(p, "/")
	}
	return strings.TrimRight(href, "/")
}

// davNormPath is the comparison form of a collection path: no trailing slash, no case (servers on case-insensitive file systems canonicalise
// the href of the collection they were asked for - WF22 F2).
func davNormPath(p string) string { return strings.ToLower(strings.TrimRight(p, "/")) }

// errPropfindTooLarge is returned when the answer exceeds the byte bound: the listing would be incomplete, so it is an error, never a truncation.
var errPropfindTooLarge = errors.New("WebDAV multistatus answer exceeds the size bound")

// boundedReader reads at most max bytes of r and then fails with errPropfindTooLarge (instead of reporting a clean EOF).
type boundedReader struct {
	r   io.Reader
	max int64
	n   int64
}

func (b *boundedReader) Read(p []byte) (int, error) {
	if b.n >= b.max {
		var one [1]byte
		if n, _ := b.r.Read(one[:]); n > 0 {
			return 0, errPropfindTooLarge
		}
		return 0, io.EOF
	}
	if int64(len(p)) > b.max-b.n {
		p = p[:b.max-b.n]
	}
	n, err := b.r.Read(p)
	b.n += int64(n)
	return n, err
}

// parsePropfind returns the children of the collection at requestURL from a PROPFIND Depth:1 multistatus body (no entry limit).
func parsePropfind(body []byte, requestURL string) ([]*FileInfo, error) {
	return parsePropfindStream(bytes.NewReader(body), requestURL, 0)
}

// parsePropfindStream returns the DIRECT children of the collection at requestURL from a PROPFIND Depth:1 multistatus read from r. The
// collection itself (compared case-insensitively and without the trailing slash) and any href that is not a direct child of it are skipped.
// Only properties of 2xx propstats count (Apache lists unknown properties in a 404 propstat). The answer is decoded response by response, and
// decoding STOPS as soon as limit entries were collected (limit < 1: no limit), so a directory far larger than the bound is never held in
// memory whole (WF22 R4/W1). The name of an entry is the decoded last segment of its href, never DAV:displayname (WF22 R10).
func parsePropfindStream(r io.Reader, requestURL string, limit int) ([]*FileInfo, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	self := ""
	if u, err := url.Parse(requestURL); err == nil {
		self = strings.TrimRight(u.Path, "/")
	}
	selfN := davNormPath(self)
	var out []*FileInfo
	sawRoot := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, errPropfindTooLarge) {
				return nil, err
			}
			return nil, fmt.Errorf("failed to parse WebDAV multistatus: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !sawRoot {
			sawRoot = true
			continue
		}
		if se.Name.Local != "response" {
			continue
		}
		var resp davResponse
		if err := dec.DecodeElement(&resp, &se); err != nil {
			if errors.Is(err, errPropfindTooLarge) {
				return nil, err
			}
			return nil, fmt.Errorf("failed to parse WebDAV multistatus: %w", err)
		}
		hp := hrefPath(resp.Href)
		// Only DIRECT children of the listed collection count (WF22 F3). This is also what drops the collection itself (a collection is never its own
		// direct child), however the server spelled its href: another case, a trailing slash, a different host (WF22 F2).
		if davNormPath(path.Dir(hp)) != selfN {
			continue
		}
		var prop davProp
		for _, ps := range resp.Propstats {
			if !davStatusOK(ps.Status) {
				continue
			}
			p := ps.Prop
			if prop.ContentLength == "" {
				prop.ContentLength = p.ContentLength
			}
			if prop.LastModified == "" {
				prop.LastModified = p.LastModified
			}
			if prop.ETag == "" {
				prop.ETag = p.ETag
			}
			if p.ResourceType.Collection != nil || p.ResourceType.Directory != nil {
				prop.ResourceType.Collection = p.ResourceType.Collection
				prop.ResourceType.Directory = p.ResourceType.Directory
			}
		}
		name := path.Base(hp)
		if name == "" || name == "." || name == "/" {
			continue
		}
		var size int64
		if n, err := strconv.ParseInt(strings.TrimSpace(prop.ContentLength), 10, 64); err == nil && n >= 0 {
			size = n
		}
		out = append(out, &FileInfo{
			Name:    name,
			Size:    size,
			ModTime: parseDAVTime(prop.LastModified),
			IsDir:   prop.ResourceType.Collection != nil || prop.ResourceType.Directory != nil,
			Mode:    0644,
			Path:    name,
		})
		if limit > 0 && len(out) >= limit {
			return out, nil
		}
	}
	if !sawRoot {
		return nil, fmt.Errorf("failed to parse WebDAV multistatus: %w", io.ErrUnexpectedEOF)
	}
	return out, nil
}
