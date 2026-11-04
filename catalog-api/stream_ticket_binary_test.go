package main

import (
	"catalogizer/internal/streamticket"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Stream tickets (security review of the web player): a <video> element cannot send an Authorization header, and putting the
// session JWT in the URL leaks it into logs and history. The web client instead mints a short-lived ticket bound to one
// media id and one user, and the stream route alone accepts it. Real server binary, real SQLite, real scan of real files.

func rawGet(t *testing.T, method, url string, hdr map[string]string) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	r, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, b, r.Header
}

func TestStreamTicket_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	media := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(media, 0o755))
	payload := func(seed byte) []byte {
		p := make([]byte, 6000)
		for i := range p {
			p[i] = byte(i*7) ^ seed
		}
		return p
	}
	require.NoError(t, os.WriteFile(filepath.Join(media, "ticketclip-alpha.mp4"), payload(1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(media, "ticketclip-bravo.mp4"), payload(2), 0o644))

	bin := filepath.Join(dir, "catalog-api")
	out0, err := execBuild(bin)
	require.NoError(t, err, "%s", out0)
	port := freePort(t)
	b := &binEnv{t: t, bin: bin, dir: dir, port: port}
	const adminPw = "Op3rator-Supplied-Value-9x"
	cmd, done, logs := b.start("GIN_MODE=release", "ADMIN_PASSWORD="+adminPw, "CATALOGIZER_READ_ROOTS="+media, "CATALOGIZER_LOCAL_ROOT_BASES="+media, "REGISTRATION_MODE=open")
	require.True(t, b.waitHealthy(done), "server must come up:\n%s", logs.String())
	defer b.stop(cmd, done)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	c := &apiClient{t: t, base: base}
	adm := c.login("admin", adminPw)

	rootID := c.id("POST", "/api/v1/storage/roots", adm, map[string]interface{}{"name": "tk", "protocol": "local", "path": media}, 201)
	code, raw := c.do("POST", "/api/v1/scans", adm, map[string]interface{}{"storage_root_id": rootID, "storage_root_name": "tk"})
	require.Less(t, code, 300, "%s", raw)
	var q struct {
		JobID string `json:"job_id"`
	}
	require.NoError(t, json.Unmarshal(raw, &q))
	require.Eventually(t, func() bool {
		_, r := c.do("GET", "/api/v1/scans", adm, nil)
		var s struct {
			Scans []struct{ JobID, Status string } `json:"scans"`
		}
		_ = json.Unmarshal(r, &s)
		for _, x := range s.Scans {
			if x.JobID == q.JobID || strings.Contains(string(r), q.JobID) {
				return strings.Contains(string(r), "completed") || strings.Contains(string(r), "failed")
			}
		}
		return false
	}, 60*time.Second, 500*time.Millisecond)
	idOf := func(name string) int {
		_, r := c.do("GET", "/api/v1/media/search?query="+name+"&limit=10", adm, nil)
		var s struct {
			Items []struct {
				ID    int
				Title string
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(r, &s))
		require.NotEmpty(t, s.Items, "scan catalogued %s: %s", name, r)
		return s.Items[0].ID
	}
	idA, idB := idOf("ticketclip-alpha"), idOf("ticketclip-bravo")
	require.NotEqual(t, idA, idB)

	plain := selfRegister(t, c, "tkplain")
	mint := func(id int, tok string) (int, string) {
		code, r := c.do("POST", fmt.Sprintf("/api/v1/stream/%d/ticket", id), tok, map[string]interface{}{})
		var m struct {
			Ticket    string `json:"ticket"`
			ExpiresIn int    `json:"expires_in"`
		}
		_ = json.Unmarshal(r, &m)
		if code == 200 {
			assert.Positive(t, m.ExpiresIn)
			assert.LessOrEqual(t, m.ExpiresIn, 60)
		}
		return code, m.Ticket
	}

	t.Run("minting requires authentication", func(t *testing.T) {
		code, _ := mint(idA, "")
		assert.Equal(t, 401, code)
	})
	t.Run("minting for a media id that does not exist is 404", func(t *testing.T) {
		code, _ := mint(987654, adm)
		assert.Equal(t, 404, code)
	})

	ticketA := ""
	t.Run("ticket for media A streams A byte-exactly with ranges, no Authorization header", func(t *testing.T) {
		code, tk := mint(idA, plain) // a plain self-registered user may stream: the catalogue is shared
		require.Equal(t, 200, code)
		require.NotEmpty(t, tk)
		ticketA = tk
		url := fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idA, tk)
		st, body, _ := rawGet(t, "GET", url, nil)
		require.Equal(t, 200, st)
		assert.Equal(t, payload(1), body)
		st, body, h := rawGet(t, "GET", url, map[string]string{"Range": "bytes=10-19"})
		assert.Equal(t, 206, st)
		assert.Equal(t, "bytes 10-19/6000", h.Get("Content-Range"))
		assert.Equal(t, payload(1)[10:20], body)
		st, _, _ = rawGet(t, "HEAD", url, nil) // the stream route is GET-only; HEAD is not routed (a ticket never widens that)
		assert.Contains(t, []int{404, 405}, st)
		st, _, _ = rawGet(t, "GET", url, map[string]string{"Range": "bytes=9000-9100"})
		assert.Equal(t, 416, st)
	})
	t.Run("ticket for A is refused on B", func(t *testing.T) {
		st, _, _ := rawGet(t, "GET", fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idB, ticketA), nil)
		assert.Equal(t, 401, st)
	})
	t.Run("ticket is not accepted on any other route or method", func(t *testing.T) {
		for _, p := range []string{
			"/api/v1/auth/me", "/api/v1/media/stats", fmt.Sprintf("/api/v1/media/%d", idA),
			fmt.Sprintf("/api/v1/download/file/%d", idA), "/api/v1/users/me"} {
			sep := "?"
			if strings.Contains(p, "?") {
				sep = "&"
			}
			st, _, _ := rawGet(t, "GET", base+p+sep+"ticket="+ticketA, nil)
			assert.Equal(t, 401, st, "GET %s with a stream ticket", p)
		}
		st, _, _ := rawGet(t, "POST", fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idA, ticketA), nil)
		assert.Contains(t, []int{401, 404, 405}, st, "POST on the stream route with a ticket")
		assert.NotEqual(t, 200, st)
	})
	t.Run("garbage and tampered tickets are 401", func(t *testing.T) {
		for _, tk := range []string{"x", "a.b", "a.b.c", ticketA[:len(ticketA)-2] + "Aa", strings.Replace(ticketA, ".", ".Z", 1)} {
			st, _, _ := rawGet(t, "GET", fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idA, tk), nil)
			assert.Equal(t, 401, st, tk)
		}
	})
	t.Run("expired and wrongly-signed tickets are 401 (forged with the real secret / a wrong one)", func(t *testing.T) {
		const secret = "Zq7-test-only-jwt-secret-0123456789abcdef"
		past := streamticket.New(secret).WithClock(func() time.Time { return time.Now().Add(-2 * time.Minute) })
		// Forge with the REAL user id and live session id of a genuinely minted ticket, so the only thing wrong with `old` is its
		// expiry (a ticket with a made-up session would be refused by the session check and prove nothing about expiry).
		code, live := mint(idA, adm)
		require.Equal(t, 200, code)
		parts := strings.Split(live, ".")
		require.Len(t, parts, 3)
		rawClaims, err := base64.RawURLEncoding.DecodeString(parts[1])
		require.NoError(t, err)
		var real streamticket.Claims
		require.NoError(t, json.Unmarshal(rawClaims, &real))
		require.NotEmpty(t, real.SessionID, "the minted ticket carries the live session id")
		fresh, _ := streamticket.New(secret).Mint(int64(idA), real.UserID, real.Username, real.SessionID)
		st, _, _ := rawGet(t, "GET", fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idA, fresh), nil)
		assert.Equal(t, 200, st, "control: same claims, same key, unexpired, is accepted")
		old, _ := past.Mint(int64(idA), real.UserID, real.Username, real.SessionID)
		st, _, _ = rawGet(t, "GET", fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idA, old), nil)
		assert.Equal(t, 401, st, "expired (everything else valid)")
		foreign, _ := streamticket.New("another-secret-entirely").Mint(int64(idA), "1", "admin", "")
		st, _, _ = rawGet(t, "GET", fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idA, foreign), nil)
		assert.Equal(t, 401, st, "wrong key")
	})
	t.Run("a ticket does not outlive its user's session", func(t *testing.T) {
		code, tk := mint(idA, plain)
		require.Equal(t, 200, code)
		code, _ = c.do("POST", "/api/v1/auth/logout", plain, map[string]interface{}{})
		require.Less(t, code, 300)
		st, _, _ := rawGet(t, "GET", fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idA, tk), nil)
		assert.Equal(t, 401, st)
	})
	t.Run("access log never contains the ticket or the access_token value", func(t *testing.T) {
		code, tk := mint(idA, adm)
		require.Equal(t, 200, code)
		rawGet(t, "GET", fmt.Sprintf("%s/api/v1/stream/%d?ticket=%s", base, idA, tk), nil)
		rawGet(t, "GET", fmt.Sprintf("%s/api/v1/stream/%d?access_token=%s", base, idA, adm), nil)
		time.Sleep(300 * time.Millisecond)
		l := logs.String()
		assert.True(t, strings.Contains(l, fmt.Sprintf("/api/v1/stream/%d", idA)), "the request itself is logged")
		assert.NotContains(t, l, tk, "ticket in log")
		assert.NotContains(t, l, ticketA, "ticket in log")
		assert.NotContains(t, l, adm, "session token in log")
	})
}

func execBuild(bin string) (string, error) {
	out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput()
	return string(out), err
}

func freePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
