package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Route-table probe (authz round 2). Skipped unless ROUTE_PROBE_FILE names a
// JSON-lines file of {method,path} rows; writes "method path anon plain admin"
// to ROUTE_PROBE_OUT. It is the measurement instrument behind
// docs/ROUTE_AUTHZ_TABLE.md; the permanent assertions live in
// route_perms_round2_binary_test.go.

var probeParam = regexp.MustCompile(`:[A-Za-z_]+|\*[A-Za-z_]+`)

func TestRouteEnumeration_Probe(t *testing.T) {
	in := os.Getenv("ROUTE_PROBE_FILE")
	if in == "" {
		t.Skip("ROUTE_PROBE_FILE not set")
	}
	c, adm, _, stop := startPermServerDir(t, t.TempDir(), t.TempDir(), "HELIX_PPROF_ENABLED=true")
	defer stop()
	plain := selfRegister(t, c, "probeplain")
	hc := &http.Client{Timeout: 4 * time.Second}
	do := func(method, path, tok string) string {
		var rd io.Reader
		if method != "GET" && method != "DELETE" {
			rd = bytes.NewReader([]byte("{}"))
		}
		req, _ := http.NewRequest(method, c.base+path, rd)
		req.Header.Set("Content-Type", "application/json")
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		r, err := hc.Do(req)
		if err != nil {
			return "ERR"
		}
		defer r.Body.Close()
		io.Copy(io.Discard, io.LimitReader(r.Body, 1<<16))
		return fmt.Sprint(r.StatusCode)
	}
	type row struct{ Method, Path string }
	var rows []row
	f, err := os.Open(in)
	require.NoError(t, err)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r row
		require.NoError(t, json.Unmarshal(sc.Bytes(), &r))
		rows = append(rows, r)
	}
	out, err := os.Create(os.Getenv("ROUTE_PROBE_OUT"))
	require.NoError(t, err)
	defer out.Close()
	// control needle: the plain token must be refused (403) - not 401 - on a known admin route,
	// before and after the run; a 401 means the token died and the column is meaningless.
	control := func(when string) {
		require.Equal(t, "403", do("POST", "/api/v1/smb/probe", plain), "control needle "+when)
	}
	control("before")
	defer control("after")
	// non-mutating first so a destructive probe cannot starve later rows
	for pass := 0; pass < 2; pass++ {
		for _, r := range rows {
			mut := r.Method != "GET"
			if r.Path == "/api/v1/auth/logout" || r.Path == "/api/v1/auth/change-password" {
				continue // would revoke / change the probe account itself
			}
			if (pass == 0) == mut {
				continue
			}
			p := probeParam.ReplaceAllString(r.Path, "1")
			p = strings.ReplaceAll(p, "/1/1", "/1/1")
			admin := "skip"
			if !mut && !strings.Contains(p, "/stream") && !strings.Contains(p, "download") {
				admin = do(r.Method, p, adm)
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", r.Method, r.Path, do(r.Method, p, ""), do(r.Method, p, plain), admin)
		}
	}
}
