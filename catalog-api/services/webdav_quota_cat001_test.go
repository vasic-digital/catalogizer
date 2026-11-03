package services

// CAT-001 review F4: GetQuota must honour each propstat <status>. RFC 4918 servers answer a property they do not support inside a 404
// propstat with an EMPTY element; decoding that as 0 (or defaulting a missing value to unlimited) fabricates quota data.
// RP3 is the reviewer's probe, adopted unchanged in substance.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func quotaServer(t *testing.T, propstats string) *WebDAVClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/</D:href>` + propstats + `</D:response></D:multistatus>`))
	}))
	t.Cleanup(srv.Close)
	return NewWebDAVClient(srv.URL, "u", "p")
}

func TestCAT001_F4_RP3_WebDAVQuota404PropstatIsNotZero(t *testing.T) {
	c := quotaServer(t, `<D:propstat><D:prop><D:quota-used-bytes/><D:quota-available-bytes/></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`)
	q, err := c.GetQuota()
	assert.ErrorIs(t, err, ErrWebDAVQuotaUnavailable)
	assert.Nil(t, q)
}

func TestCAT001_F4_OnlyOnePropertyReportedIsUnavailableNotUnlimited(t *testing.T) {
	c := quotaServer(t, `<D:propstat><D:prop><D:quota-used-bytes>100</D:quota-used-bytes></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`+
		`<D:propstat><D:prop><D:quota-available-bytes/></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`)
	q, err := c.GetQuota()
	assert.ErrorIs(t, err, ErrWebDAVQuotaUnavailable, "available is unknown, which is not -1 / unlimited")
	assert.Nil(t, q)
}

func TestCAT001_F4_EmptyElementInOKPropstatIsUnavailable(t *testing.T) {
	c := quotaServer(t, `<D:propstat><D:prop><D:quota-used-bytes/><D:quota-available-bytes/></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
	q, err := c.GetQuota()
	assert.ErrorIs(t, err, ErrWebDAVQuotaUnavailable)
	assert.Nil(t, q)
}

func TestCAT001_F4_MissingStatusIsNotTrusted(t *testing.T) {
	c := quotaServer(t, `<D:propstat><D:prop><D:quota-used-bytes>5</D:quota-used-bytes><D:quota-available-bytes>7</D:quota-available-bytes></D:prop></D:propstat>`)
	_, err := c.GetQuota()
	assert.ErrorIs(t, err, ErrWebDAVQuotaUnavailable)
}

func TestCAT001_F4_BothReportedWith200IsReturned(t *testing.T) {
	c := quotaServer(t, `<D:propstat><D:prop><D:quota-used-bytes>1234</D:quota-used-bytes><D:quota-available-bytes>8766</D:quota-available-bytes></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
	q, err := c.GetQuota()
	require.NoError(t, err)
	assert.EqualValues(t, 1234, q.Used)
	assert.EqualValues(t, 8766, q.Available)
}

func TestCAT001_F4_NonNumericValueIsUnavailable(t *testing.T) {
	c := quotaServer(t, `<D:propstat><D:prop><D:quota-used-bytes>lots</D:quota-used-bytes><D:quota-available-bytes>8</D:quota-available-bytes></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
	_, err := c.GetQuota()
	assert.ErrorIs(t, err, ErrWebDAVQuotaUnavailable)
}
