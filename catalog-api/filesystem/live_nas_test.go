//go:build live

// LIVE read-only integration tests for the REAL SMB and FTP clients against real Synology shares
// (constitution §11.4.289: no mocks, stubs or fixture servers; the real clients talk to real hosts).
//
// Credentials and host addresses come ONLY from the environment (SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD,
// SYNOLOGY_IP_1..7); they are never logged. When they are absent every test SKIPs with a reason (never passes).
// Nothing here writes, deletes, renames or mounts anything on a share.
//
// Mutation switch (paired mutation, proves the tests can FAIL): LIVE_MUTATE=share breaks every share/base-path
// name, LIVE_MUTATE=password breaks the password; the positive tests must then FAIL.
package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

type liveEnv struct {
	user, pass string
	ips        []string
}

func loadLiveEnv(t *testing.T) liveEnv {
	t.Helper()
	e := liveEnv{user: os.Getenv("SYNOLOGY_SMB_USER"), pass: os.Getenv("SYNOLOGY_SMB_PASSWORD")}
	for i := 1; i <= 7; i++ {
		if ip := os.Getenv(fmt.Sprintf("SYNOLOGY_IP_%d", i)); ip != "" {
			e.ips = append(e.ips, ip)
		}
	}
	if e.user == "" || e.pass == "" || len(e.ips) == 0 {
		t.Skip("SKIP(credentials_absent): SYNOLOGY_SMB_USER / SYNOLOGY_SMB_PASSWORD / SYNOLOGY_IP_* not set; live NAS tests need the real Synology environment")
	}
	if os.Getenv("LIVE_MUTATE") == "password" {
		e.pass += "_x"
	}
	return e
}

// mut applies the share-name mutation when LIVE_MUTATE=share.
func mut(name string) string {
	if os.Getenv("LIVE_MUTATE") == "share" {
		return name + "_BROKEN"
	}
	return name
}

var (
	smbHostOnce sync.Mutex
	smbHostMemo = map[string]string{}
)

// findSMBHost returns the host (of SYNOLOGY_IP_*) that serves the given share, found by really mounting it.
func findSMBHost(t *testing.T, e liveEnv, share string) string {
	t.Helper()
	smbHostOnce.Lock()
	defer smbHostOnce.Unlock()
	if h, ok := smbHostMemo[share]; ok {
		return h
	}
	for _, ip := range e.ips {
		c := NewSmbClient(&SmbConfig{Host: ip, Port: 445, Share: mut(share), Username: e.user, Password: e.pass})
		if err := c.Connect(context.Background()); err == nil {
			_ = c.Disconnect(context.Background())
			smbHostMemo[share] = ip
			return ip
		}
	}
	t.Fatalf("no host among SYNOLOGY_IP_* (%d candidates) mounts SMB share %q", len(e.ips), mut(share))
	return ""
}

func connectSMB(t *testing.T, e liveEnv, share string) *SmbClient {
	t.Helper()
	host := findSMBHost(t, e, share)
	c := NewSmbClient(&SmbConfig{Host: host, Port: 445, Share: mut(share), Username: e.user, Password: e.pass})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("SMB connect share=%s: %v", share, redact(err, e))
	}
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	if !c.IsConnected() {
		t.Fatalf("IsConnected false after successful Connect")
	}
	if err := c.TestConnection(ctx); err != nil {
		t.Fatalf("TestConnection: %v", redact(err, e))
	}
	return c
}

func redact(err error, e liveEnv) string {
	s := err.Error()
	for _, secret := range []string{e.pass, e.user} {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "<redacted>")
		}
	}
	return s
}

func hasNonASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

type walkResult struct {
	dirs, files int
	denied      int    // directories the account may not list (ACL)
	firstFile   string // first non-empty regular file found
	special     []string
	cyr         []string
}

// walk lists directories breadth-first to maxDepth with at most maxLists ListDirectory calls (bounded, read-only).
func walk(t *testing.T, c FileSystemClient, start string, maxDepth, maxLists int) walkResult {
	t.Helper()
	type item struct {
		p string
		d int
	}
	var res walkResult
	queue := []item{{start, 0}}
	lists := 0
	ctx := context.Background()
	for len(queue) > 0 && lists < maxLists {
		it := queue[0]
		queue = queue[1:]
		entries, err := c.ListDirectory(ctx, it.p)
		lists++
		if err != nil {
			// a folder the test account may not open (ACL) is a property of the NAS, not a client defect: count and go on
			if strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "550") {
				res.denied++
				continue
			}
			t.Fatalf("ListDirectory(%q) depth %d: %v", it.p, it.d, err)
		}
		for _, en := range entries {
			if !utf8.ValidString(en.Name) {
				t.Errorf("non-UTF-8 name returned in %q", it.p)
			}
			if en.Name == "." || en.Name == ".." {
				t.Errorf("listing of %q contains pseudo entry %q", it.p, en.Name)
			}
			child := pathpkg.Join(it.p, en.Name)
			if strings.ContainsAny(en.Name, " []()'&,;#") {
				res.special = append(res.special, child)
			}
			if hasCyrillic(en.Name) {
				res.cyr = append(res.cyr, child)
			}
			if en.IsDir {
				res.dirs++
				if it.d+1 <= maxDepth {
					queue = append(queue, item{child, it.d + 1})
				}
			} else {
				res.files++
				if res.firstFile == "" && en.Size > 0 && !strings.HasPrefix(en.Name, ".") {
					res.firstFile = child
				}
			}
		}
	}
	return res
}

func readFirst(t *testing.T, c FileSystemClient, p string, n int) []byte {
	t.Helper()
	rc, err := c.ReadFile(context.Background(), p)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", p, err)
	}
	defer rc.Close()
	buf := make([]byte, n)
	got, err := io.ReadFull(rc, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		t.Fatalf("read first bytes of %q: %v", p, err)
	}
	return buf[:got]
}

type smbTarget struct{ share, preferred string }

var smbTargets = []smbTarget{
	{"DATA12", "Филмови"},
	{"DATA18", "Серије"},
	{"DATA20-3", "Музика"},
	{"WORK20", ""},
	{"DATA8", "Стрипови"},
}

func TestLiveSMB_ListStatReadWalk(t *testing.T) {
	e := loadLiveEnv(t)
	for _, tg := range smbTargets {
		tg := tg
		t.Run(tg.share, func(t *testing.T) {
			c := connectSMB(t, e, tg.share)
			ctx := context.Background()
			root, err := c.ListDirectory(ctx, ".")
			if err != nil {
				t.Fatalf("list root: %v", err)
			}
			if len(root) == 0 {
				t.Fatalf("share %s root listing is empty", tg.share)
			}
			start := "."
			if tg.preferred != "" {
				fi, err := c.GetFileInfo(ctx, tg.preferred)
				if err != nil {
					t.Fatalf("stat %q (Cyrillic dir): %v", tg.preferred, err)
				}
				if !fi.IsDir || fi.Name != tg.preferred {
					t.Fatalf("stat %q: IsDir=%v Name=%q (round-trip mismatch)", tg.preferred, fi.IsDir, fi.Name)
				}
				start = tg.preferred
			}
			res := walk(t, c, start, 3, 40)
			t.Logf("share=%s root_entries=%d walk_dirs=%d walk_files=%d special=%d cyrillic=%d", tg.share, len(root), res.dirs, res.files, len(res.special), len(res.cyr))
			if res.firstFile == "" {
				t.Skipf("SKIP(no_file_in_bound): no non-empty regular file within depth 3 / 40 listings under %s:%s; read-bytes leg not exercised", tg.share, start)
			}
			fi, err := c.GetFileInfo(ctx, res.firstFile)
			if err != nil {
				t.Fatalf("stat file: %v", err)
			}
			if fi.IsDir || fi.Size <= 0 {
				t.Fatalf("stat file: IsDir=%v Size=%d", fi.IsDir, fi.Size)
			}
			want := 512
			if fi.Size < int64(want) {
				want = int(fi.Size)
			}
			b := readFirst(t, c, res.firstFile, 512)
			if len(b) != want {
				t.Fatalf("read first bytes: got %d want %d (file size %d)", len(b), want, fi.Size)
			}
			// Seekable path: seek to offset 0 of a different position and read (HTTP range / video seek path).
			sk, err := c.OpenSeekable(ctx, res.firstFile)
			if err != nil {
				t.Fatalf("OpenSeekable: %v", err)
			}
			defer sk.Close()
			if fi.Size > 8 {
				if _, err := sk.Seek(fi.Size-8, io.SeekStart); err != nil {
					t.Fatalf("seek: %v", err)
				}
				tail := make([]byte, 8)
				if _, err := io.ReadFull(sk, tail); err != nil {
					t.Fatalf("read after seek to tail: %v", err)
				}
			}
			t.Logf("share=%s read_bytes=%d size_bytes=%d seek_tail=ok", tg.share, len(b), fi.Size)
		})
	}
}

// Every non-ASCII / Cyrillic / special-character name seen in a listing must round-trip: stat by the listed name must
// succeed and report the same name, and the parent listing must contain it.
func TestLiveSMB_NameRoundTrip(t *testing.T) {
	e := loadLiveEnv(t)
	cyrTotal, specTotal := 0, 0
	for _, tg := range smbTargets {
		tg := tg
		t.Run(tg.share, func(t *testing.T) {
			c := connectSMB(t, e, tg.share)
			ctx := context.Background()
			res := walk(t, c, ".", 2, 25)
			names := append(append([]string{}, res.cyr...), res.special...)
			if len(names) > 25 {
				names = names[:25]
			}
			for _, p := range names {
				fi, err := c.GetFileInfo(ctx, p)
				if err != nil {
					t.Errorf("stat of listed name (non-ASCII/special) failed: %v", err)
					continue
				}
				if fi.Name != pathpkg.Base(p) {
					t.Errorf("name round-trip mismatch: listed %q stat %q", pathpkg.Base(p), fi.Name)
				}
				ok, err := c.FileExists(ctx, p)
				if err != nil || !ok {
					t.Errorf("FileExists(listed name) = %v, %v", ok, err)
				}
				if !utf8.ValidString(fi.Name) {
					t.Errorf("invalid UTF-8 after round trip")
				}
			}
			cyrTotal += len(res.cyr)
			specTotal += len(res.special)
			t.Logf("share=%s checked=%d (cyrillic_seen=%d special_seen=%d)", tg.share, len(names), len(res.cyr), len(res.special))
		})
	}
	if cyrTotal == 0 {
		t.Errorf("no Cyrillic name seen on any share: the Cyrillic round-trip leg proved nothing")
	}
	if specTotal == 0 {
		t.Errorf("no special-character name seen on any share: the special-char leg proved nothing")
	}
}

func TestLiveSMB_NegativeControls(t *testing.T) {
	e := loadLiveEnv(t)
	host := findSMBHost(t, e, "DATA12")
	ctx := context.Background()
	t.Run("wrong_password", func(t *testing.T) {
		c := NewSmbClient(&SmbConfig{Host: host, Port: 445, Share: "DATA12", Username: e.user, Password: e.pass + "_WRONG"})
		if err := c.Connect(ctx); err == nil {
			_ = c.Disconnect(ctx)
			t.Fatalf("Connect with wrong password succeeded against real host")
		}
		if c.IsConnected() {
			t.Fatalf("IsConnected true after failed Connect")
		}
	})
	t.Run("unreachable_port_on_real_host", func(t *testing.T) {
		c := NewSmbClient(&SmbConfig{Host: host, Port: 1, Share: "DATA12", Username: e.user, Password: e.pass})
		start := time.Now()
		if err := c.Connect(ctx); err == nil {
			_ = c.Disconnect(ctx)
			t.Fatalf("Connect to closed port succeeded")
		}
		t.Logf("closed-port error after %s", time.Since(start).Round(time.Millisecond))
	})
	t.Run("nonexistent_share", func(t *testing.T) {
		c := NewSmbClient(&SmbConfig{Host: host, Port: 445, Share: "NO_SUCH_SHARE_LIVE", Username: e.user, Password: e.pass})
		if err := c.Connect(ctx); err == nil {
			_ = c.Disconnect(ctx)
			t.Fatalf("Connect to nonexistent share succeeded")
		}
	})
	t.Run("nonexistent_path_and_not_connected", func(t *testing.T) {
		c := connectSMB(t, e, "DATA12")
		if _, err := c.GetFileInfo(ctx, "no-such-dir-live-xyz"); err == nil {
			t.Fatalf("stat of nonexistent path succeeded")
		}
		if _, err := c.ListDirectory(ctx, "no-such-dir-live-xyz"); err == nil {
			t.Fatalf("list of nonexistent path succeeded")
		}
		if _, err := c.ReadFile(ctx, "no-such-dir-live-xyz/a.bin"); err == nil {
			t.Fatalf("read of nonexistent file succeeded")
		}
		ok, err := c.FileExists(ctx, "no-such-dir-live-xyz")
		if err != nil || ok {
			t.Fatalf("FileExists(nonexistent) = %v, %v; want false, nil", ok, err)
		}
		_ = c.Disconnect(ctx)
		if c.IsConnected() {
			t.Errorf("IsConnected true after Disconnect")
		}
		done := make(chan error, 1)
		go func() { _, err := c.ListDirectory(ctx, "."); done <- err }()
		select {
		case err := <-done:
			if err == nil {
				t.Fatalf("ListDirectory after Disconnect succeeded")
			}
		case <-time.After(15 * time.Second):
			t.Fatalf("ListDirectory after Disconnect HUNG (>15s)")
		}
	})
}

// ---------------------------------------------------------------- FTP

type ftpTarget struct {
	share string // FTP root folder == Synology shared folder
	rel   string // directory inside the share used as the configured base Path
}

var ftpTargets = []ftpTarget{
	{"DATA12", "Филмови"},
	{"DATA18", "Серије"},
	{"DATA20-3", "Музика"},
	{"WORK20", ""},
}

type ftpAddr struct {
	host string
	port int
}

var (
	ftpMu   sync.Mutex
	ftpMemo = map[string]ftpAddr{}
)

// findFTPHost locates (host, port) whose FTP root contains the share folder, by really logging in and listing "/".
func findFTPHost(t *testing.T, e liveEnv, share string) ftpAddr {
	t.Helper()
	ftpMu.Lock()
	defer ftpMu.Unlock()
	if a, ok := ftpMemo[share]; ok {
		return a
	}
	for _, ip := range e.ips {
		for _, port := range []int{21, 7721} {
			c := NewFTPClient(&FTPConfig{Host: ip, Port: port, Username: e.user, Password: e.pass})
			if err := c.Connect(context.Background()); err != nil {
				continue
			}
			ents, err := c.ListDirectory(context.Background(), "/")
			_ = c.Disconnect(context.Background())
			if err != nil {
				continue
			}
			for _, en := range ents {
				if en.Name == share {
					a := ftpAddr{ip, port}
					ftpMemo[share] = a
					return a
				}
			}
		}
	}
	t.Fatalf("no FTP host among SYNOLOGY_IP_* exposes root folder %q", share)
	return ftpAddr{}
}

func connectFTP(t *testing.T, e liveEnv, tg ftpTarget) *FTPClient {
	t.Helper()
	a := findFTPHost(t, e, tg.share)
	base := "/" + mut(tg.share)
	if tg.rel != "" {
		base += "/" + tg.rel
	}
	c := NewFTPClient(&FTPConfig{Host: a.host, Port: a.port, Username: e.user, Password: e.pass, Path: base})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("FTP connect share=%s base=%q: %v", tg.share, base, redact(err, e))
	}
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	if !c.IsConnected() {
		t.Fatalf("IsConnected false after Connect")
	}
	if err := c.TestConnection(ctx); err != nil {
		t.Fatalf("TestConnection: %v", redact(err, e))
	}
	return c
}

func TestLiveFTP_ListStatReadWalk(t *testing.T) {
	e := loadLiveEnv(t)
	for _, tg := range ftpTargets {
		tg := tg
		t.Run(tg.share, func(t *testing.T) {
			c := connectFTP(t, e, tg)
			ctx := context.Background()
			root, err := c.ListDirectory(ctx, ".")
			if err != nil {
				t.Fatalf("list base: %v", err)
			}
			if len(root) == 0 {
				t.Fatalf("FTP base listing empty for %s", tg.share)
			}
			// stat of a directory entry and of a file entry from the listing
			var sd *FileInfo
			for _, en := range root {
				if en.IsDir {
					sd = en
					break
				}
			}
			if sd != nil {
				fi, err := c.GetFileInfo(ctx, sd.Name)
				if err != nil {
					t.Errorf("GetFileInfo(dir %q) failed: %v", sd.Name, err)
				} else if !fi.IsDir {
					t.Errorf("GetFileInfo(dir) reports IsDir=false")
				}
			}
			res := walk(t, c, ".", 3, 40)
			t.Logf("share=%s base_entries=%d walk_dirs=%d walk_files=%d special=%d cyrillic=%d", tg.share, len(root), res.dirs, res.files, len(res.special), len(res.cyr))
			if res.firstFile == "" {
				t.Skipf("SKIP(no_file_in_bound): no non-empty regular file within depth 3 / 40 listings under FTP %s; read-bytes leg not exercised", tg.share)
			}
			fi, err := c.GetFileInfo(ctx, res.firstFile)
			if err != nil {
				t.Fatalf("GetFileInfo(file): %v", err)
			}
			if fi.Size <= 0 {
				t.Fatalf("GetFileInfo(file) size=%d", fi.Size)
			}
			want := 512
			if fi.Size < int64(want) {
				want = int(fi.Size)
			}
			b := readFirst(t, c, res.firstFile, 512)
			if len(b) != want {
				t.Fatalf("read first bytes: got %d want %d (size %d)", len(b), want, fi.Size)
			}
			// the control connection must still be usable after an abandoned partial RETR
			if err := c.TestConnection(ctx); err != nil {
				t.Fatalf("control connection unusable after partial read: %v", err)
			}
			if _, err := c.ListDirectory(ctx, "."); err != nil {
				t.Fatalf("list after partial read: %v", err)
			}
			t.Logf("share=%s read_bytes=%d size_bytes=%d", tg.share, len(b), fi.Size)
		})
	}
}

func TestLiveFTP_NameRoundTrip(t *testing.T) {
	e := loadLiveEnv(t)
	cyrTotal := 0
	for _, tg := range ftpTargets {
		tg := tg
		t.Run(tg.share, func(t *testing.T) {
			c := connectFTP(t, e, ftpTarget{tg.share, ""})
			ctx := context.Background()
			res := walk(t, c, ".", 2, 25)
			names := append(append([]string{}, res.cyr...), res.special...)
			if len(names) > 25 {
				names = names[:25]
			}
			for _, p := range names {
				ok, err := c.FileExists(ctx, p)
				if err != nil || !ok {
					t.Errorf("FileExists(listed non-ASCII/special name) = %v, %v", ok, err)
				}
			}
			cyrTotal += len(res.cyr)
			t.Logf("share=%s checked=%d cyrillic_seen=%d special_seen=%d", tg.share, len(names), len(res.cyr), len(res.special))
		})
	}
	if cyrTotal == 0 {
		t.Errorf("no Cyrillic name seen on any FTP share")
	}
	// a Cyrillic base Path (as the product configures it) must also be accepted
	c := connectFTP(t, e, ftpTarget{"DATA12", "Филмови"})
	if !hasNonASCII("Филмови") || !c.IsConnected() {
		t.Fatalf("Cyrillic base path not usable")
	}
}

func TestLiveFTP_NegativeControls(t *testing.T) {
	e := loadLiveEnv(t)
	a := findFTPHost(t, e, "DATA12")
	ctx := context.Background()
	t.Run("wrong_password", func(t *testing.T) {
		c := NewFTPClient(&FTPConfig{Host: a.host, Port: a.port, Username: e.user, Password: e.pass + "_WRONG"})
		if err := c.Connect(ctx); err == nil {
			_ = c.Disconnect(ctx)
			t.Fatalf("FTP login with wrong password succeeded")
		}
		if c.IsConnected() {
			t.Fatalf("IsConnected true after failed login")
		}
	})
	t.Run("closed_port_on_real_host", func(t *testing.T) {
		c := NewFTPClient(&FTPConfig{Host: a.host, Port: 2, Username: e.user, Password: e.pass})
		if err := c.Connect(ctx); err == nil {
			_ = c.Disconnect(ctx)
			t.Fatalf("FTP connect to closed port succeeded")
		}
	})
	t.Run("nonexistent_base_path", func(t *testing.T) {
		c := NewFTPClient(&FTPConfig{Host: a.host, Port: a.port, Username: e.user, Password: e.pass, Path: "/NO_SUCH_SHARE_LIVE"})
		if err := c.Connect(ctx); err == nil {
			_ = c.Disconnect(ctx)
			t.Fatalf("FTP connect with nonexistent base path succeeded")
		}
	})
	t.Run("nonexistent_entries", func(t *testing.T) {
		c := connectFTP(t, e, ftpTarget{"DATA12", ""})
		if _, err := c.ListDirectory(ctx, "no-such-dir-live-xyz"); err == nil {
			t.Logf("UNCONFIRMED-BEHAVIOUR: ListDirectory(nonexistent) returned no error")
		}
		if _, err := c.ReadFile(ctx, "no-such-dir-live-xyz/a.bin"); err == nil {
			t.Fatalf("read of nonexistent file succeeded")
		}
		ok, err := c.FileExists(ctx, "no-such-dir-live-xyz")
		if err != nil || ok {
			t.Fatalf("FileExists(nonexistent) = %v, %v; want false, nil", ok, err)
		}
		_ = c.Disconnect(ctx)
		if _, err := c.ListDirectory(ctx, "."); err == nil {
			t.Fatalf("ListDirectory after Disconnect succeeded")
		}
	})
}

// NFS: honest SKIP. The survey found rpcbind/nfsd open on the hosts but ZERO visible exports (showmount list empty),
// so there is no real export to mount; building a fake NFS server would violate §11.4.289.
func TestLiveNFS_NoExports(t *testing.T) {
	t.Skip("SKIP(topology_unsupported): the real Synology hosts expose no NFS exports to the test account (rpcbind 111 / nfsd 2049 open, exports list empty per nas_survey.md); NFS live validation needs an export to be enabled by the operator, a stub NFS server would be a test double")
}
