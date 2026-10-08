package filesystem

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PA-03: the PROPFIND parser against the shapes real servers emit. The Apache sample is the structure the repository's own WebDAV fixture
// (bytemark/webdav, Apache mod_dav) answers: per-response namespace declarations, lp1: prefixes, and a 404 propstat for unknown properties.
const apacheMultistatus = `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:ns0="DAV:">
<D:response xmlns:lp1="DAV:" xmlns:lp2="http://apache.org/dav/props/" xmlns:g0="DAV:">
<D:href>/media/</D:href>
<D:propstat><D:prop><lp1:getlastmodified>Tue, 14 Nov 2023 22:13:20 GMT</lp1:getlastmodified><lp1:resourcetype><D:collection/></lp1:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>
<D:propstat><D:prop><g0:displayname/><g0:getcontentlength/></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>
</D:response>
<D:response xmlns:lp1="DAV:" xmlns:lp2="http://apache.org/dav/props/" xmlns:g0="DAV:">
<D:href>/media/music/</D:href>
<D:propstat><D:prop><lp1:getlastmodified>Tue, 14 Nov 2023 22:13:20 GMT</lp1:getlastmodified><lp1:resourcetype><D:collection/></lp1:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>
<D:propstat><D:prop><g0:displayname/><g0:getcontentlength/></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>
</D:response>
<D:response xmlns:lp1="DAV:" xmlns:lp2="http://apache.org/dav/props/" xmlns:g0="DAV:">
<D:href>/media/Bj%C3%B6rk%20-%20J%C3%B3ga.flac</D:href>
<D:propstat><D:prop><lp1:getcontentlength>4096</lp1:getcontentlength><lp1:getlastmodified>Wed, 15 Nov 2023 01:02:03 GMT</lp1:getlastmodified><lp1:getetag>"1000-abc"</lp1:getetag><lp1:resourcetype/></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>
</D:response>
</D:multistatus>`

func TestParsePropfind_ApacheModDav(t *testing.T) {
	files, err := parsePropfind([]byte(apacheMultistatus), "http://webdav:80/media/")
	require.NoError(t, err)
	require.Len(t, files, 2, "the collection itself is skipped")
	assert.Equal(t, "music", files[0].Name)
	assert.True(t, files[0].IsDir)
	assert.Equal(t, int64(0), files[0].Size)
	assert.Equal(t, time.Date(2023, 11, 14, 22, 13, 20, 0, time.UTC), files[0].ModTime.UTC())
	assert.Equal(t, "Björk - Jóga.flac", files[1].Name, "an encoded href is decoded and its last segment becomes the name")
	assert.False(t, files[1].IsDir)
	assert.Equal(t, int64(4096), files[1].Size)
	assert.Equal(t, "Björk - Jóga.flac", files[1].Path)
}

func TestParsePropfind_PrefixesAndAbsoluteHrefs(t *testing.T) {
	body := `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">
<d:response><d:href>http://h:8080/dav/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
<d:response><d:href>http://h:8080/dav/a.txt</d:href><d:propstat><d:prop><d:displayname>a.txt</d:displayname><d:getcontentlength>7</d:getcontentlength><d:resourcetype/></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>
</d:multistatus>`
	files, err := parsePropfind([]byte(body), "http://h:8080/dav")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "a.txt", files[0].Name)
	assert.Equal(t, int64(7), files[0].Size)
	assert.True(t, files[0].ModTime.IsZero(), "no getlastmodified: the zero time, never 'now' (it feeds change detection)")
}

func TestParsePropfind_IgnoresPropertiesOfFailedPropstats(t *testing.T) {
	// the failed propstat comes FIRST: a parser that merges every propstat would take its values
	body := `<D:multistatus xmlns:D="DAV:"><D:response><D:href>/x/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>
<D:response><D:href>/x/f</D:href>
<D:propstat><D:prop><D:getcontentlength>999</D:getcontentlength><D:displayname>forged-name</D:displayname><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 403 Forbidden</D:status></D:propstat>
<D:propstat><D:prop><D:getcontentlength>5</D:getcontentlength></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`
	files, err := parsePropfind([]byte(body), "http://h/x/")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, int64(5), files[0].Size)
	assert.Equal(t, "f", files[0].Name, "the display name of a failed propstat is not used")
	assert.False(t, files[0].IsDir, "the resource type of a failed propstat is not used")
}

func TestParsePropfind_EmptyAndMalformed(t *testing.T) {
	files, err := parsePropfind([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"></D:multistatus>`), "http://h/")
	require.NoError(t, err)
	assert.Empty(t, files)
	_, err = parsePropfind([]byte(`<html><body>not xml at all`), "http://h/")
	assert.Error(t, err, "a non-multistatus body must be an error, never an empty listing")
	_, err = parsePropfind([]byte(``), "http://h/")
	assert.Error(t, err)
}

func TestParsePropfind_NegativeAndGarbageLengthBecomeZero(t *testing.T) {
	body := `<D:multistatus xmlns:D="DAV:"><D:response><D:href>/r/</D:href></D:response>
<D:response><D:href>/r/a</D:href><D:propstat><D:prop><D:getcontentlength>-4</D:getcontentlength></D:prop></D:propstat></D:response>
<D:response><D:href>/r/b</D:href><D:propstat><D:prop><D:getcontentlength>abc</D:getcontentlength></D:prop></D:propstat></D:response></D:multistatus>`
	files, err := parsePropfind([]byte(body), "http://h/r/")
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, int64(0), files[0].Size)
	assert.Equal(t, int64(0), files[1].Size)
}

func TestDavStatusOK(t *testing.T) {
	for s, want := range map[string]bool{"": true, "HTTP/1.1 200 OK": true, "HTTP/1.1 207 Multi-Status": true, "HTTP/1.1 404 Not Found": false, "HTTP/1.1 403 Forbidden": false, "garbage": false, "HTTP/1.1 abc": false} {
		assert.Equal(t, want, davStatusOK(s), s)
	}
}
