package services

// CAT-001: connectivity / capability checks that answered "ok" without checking anything. Each test drives the real code against real
// sockets / real HTTP servers / the real PATH lookup.

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCAT001_WizardNetworkDatabaseTest_ReallyDials(t *testing.T) {
	svc := NewConfigurationWizardService(nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	cfg := func(p int) map[string]interface{} {
		return map[string]interface{}{"db_host": "127.0.0.1", "db_port": p, "db_name": "d", "db_username": "u", "db_password": "p"}
	}
	assert.NoError(t, svc.testNetworkDatabaseConnection(cfg(port)), "a listening server is reachable")
	ln.Close()
	err = svc.testNetworkDatabaseConnection(cfg(port))
	require.Error(t, err, "a closed port must not pass the connection test")
	assert.NotContains(t, err.Error(), "p\"", "no password in the error")
	// the port may also arrive as a string or float64 (JSON)
	assert.Error(t, svc.testNetworkDatabaseConnection(map[string]interface{}{"db_host": "127.0.0.1", "db_port": strconv.Itoa(port), "db_name": "d", "db_username": "u", "db_password": "p"}))
	assert.Error(t, svc.testNetworkDatabaseConnection(map[string]interface{}{"db_host": "127.0.0.1", "db_port": "not-a-port", "db_name": "d", "db_username": "u", "db_password": "p"}))
}

func TestCAT001_DetectInstalledTools_UsesThePATH(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"git", "node"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\n"), 0o755))
	}
	t.Setenv("PATH", dir)
	tools := NewConfigurationWizardService(nil).detectInstalledTools()
	assert.ElementsMatch(t, []string{"git", "node"}, tools, "only what is really on PATH; go is not assumed")
}

func TestCAT001_SyncTestConnection_CloudStorageIsNotReportedReachable(t *testing.T) {
	svc := NewSyncService(nil, nil, nil)
	err := svc.testConnection(&models.SyncEndpoint{Type: models.SyncTypeCloudStorage, URL: "s3://bucket"})
	require.Error(t, err, "no connectivity test exists for cloud storage; success must not be fabricated")
	assert.Nil(t, svc.testConnection(&models.SyncEndpoint{Type: models.SyncTypeLocal, URL: "file:///tmp/x"}), "local endpoints have no remote to test")
}

func TestCAT001_WebDAVGetQuota_ReadsTheServersQuota(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/</D:href><D:propstat><D:prop>` +
			`<D:quota-used-bytes>1234</D:quota-used-bytes><D:quota-available-bytes>5678</D:quota-available-bytes></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
	}))
	defer srv.Close()
	q, err := NewWebDAVClient(srv.URL, "u", "p").GetQuota()
	require.NoError(t, err)
	assert.EqualValues(t, 1234, q.Used)
	assert.EqualValues(t, 5678, q.Available)
}

func TestCAT001_WebDAVGetQuota_ServerWithoutQuotaIsAnErrorNotUnlimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`))
	}))
	defer srv.Close()
	q, err := NewWebDAVClient(srv.URL, "u", "p").GetQuota()
	require.Error(t, err, "a server that reports no quota is not 'unlimited'")
	assert.Nil(t, q)
}
