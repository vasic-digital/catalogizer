//go:build live

// LIVE read-only integration tests for the REAL SMB and FTP clients against real Synology shares
// (constitution §11.4.289: no mocks, stubs or fixture servers; the real clients talk to real hosts).
//
// Credentials and host addresses come ONLY from the environment (SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD,
// SYNOLOGY_IP_1..7); they are never logged. When they are absent every test SKIPs with a reason (never passes).
// Nothing here writes, deletes, renames or mounts anything on a share.
//
// Environment (names only; values live in the operator's private .env, see .env.example):
//
//	SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD, SYNOLOGY_IP_1..SYNOLOGY_IP_7   credentials and host addresses
//	LIVE_SMB_SHARES            "SHARE:folder,SHARE:folder" SMB shares and the Cyrillic folder to start in (default: the survey set)
//	LIVE_SMB_OPTIONAL_SHARES   shares that are exercised only when some host mounts them (default DATA22-2)
//	LIVE_FTP_SHARES            "SHARE:folder,..." FTP root folders and the base folder inside each
//	LIVE_SPECIAL_REQUIRED      shares where the survey found ASCII special-character names; zero such names checked there is a FAIL
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
	"regexp"
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
	liveSecrets = append([]string{e.user, e.pass}, e.ips...)
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

var (
	liveSecrets []string
	ipv4        = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
)

// rd renders an error for test output with credentials and host addresses removed.
func rd(err error) string {
	if err == nil {
		return "<nil>"
	}
	s := err.Error()
	for _, secret := range liveSecrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "<redacted>")
		}
	}
	return ipv4.ReplaceAllString(s, "<ip>")
}

func redact(err error, e liveEnv) string { return rd(err) }

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
			t.Fatalf("ListDirectory(%q) depth %d: %v", it.p, it.d, rd(err))
		}
		for _, en := range entries {
			if !utf8.ValidString(en.Name) {
				t.Errorf("non-UTF-8 name returned in %q", it.p)
			}
			if en.Name == "." || en.Name == ".." {
				t.Errorf("listing of %q contains pseudo entry %q", it.p, en.Name)
			}
			child := pathpkg.Join(it.p, en.Name)
			if !hasNonASCII(en.Name) && strings.ContainsAny(en.Name, "[]()'&,;#!@$%+=~") {
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
		t.Fatalf("ReadFile(%q): %v", p, rd(err))
	}
	buf := make([]byte, n)
	got, err := io.ReadFull(rc, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		_ = rc.Close()
		t.Fatalf("read first bytes of %q: %v", p, rd(err))
	}
	// Close semantics: an early close after a deliberate partial read must return nil (the session survives, see ftpReadCloser).
	if cerr := rc.Close(); cerr != nil {
		t.Errorf("Close() after reading the first %d bytes = %v, want nil", got, rd(cerr))
	}
	return buf[:got]
}

type smbTarget struct {
	share, preferred string
	optional         bool
}

func parseTargets(env, def string) [][2]string {
	v := os.Getenv(env)
	if v == "" {
		v = def
	}
	var out [][2]string
	for _, it := range strings.Split(v, ",") {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		parts := strings.SplitN(it, ":", 2)
		f := ""
		if len(parts) == 2 {
			f = parts[1]
		}
		out = append(out, [2]string{parts[0], f})
	}
	return out
}

func smbTargets() []smbTarget {
	var out []smbTarget
	for _, kv := range parseTargets("LIVE_SMB_SHARES", "DATA12:Филмови,DATA18:Серије,DATA20-3:Музика,WORK20:,DATA8:Стрипови") {
		out = append(out, smbTarget{share: kv[0], preferred: kv[1]})
	}
	for _, kv := range parseTargets("LIVE_SMB_OPTIONAL_SHARES", "DATA22-2") {
		out = append(out, smbTarget{share: kv[0], preferred: kv[1], optional: true})
	}
	return out
}

func specialRequired(share string) bool {
	v := os.Getenv("LIVE_SPECIAL_REQUIRED")
	if v == "" {
		v = "WORK20"
	}
	for _, x := range strings.Split(v, ",") {
		if strings.TrimSpace(x) == share {
			return true
		}
	}
	return false
}

// skipIfOptionalAbsent skips an optional target when no host mounts the share (an honest SKIP, never a pass).
func skipIfOptionalAbsent(t *testing.T, e liveEnv, tg smbTarget) {
	t.Helper()
	if !tg.optional {
		return
	}
	for _, ip := range e.ips {
		c := NewSmbClient(&SmbConfig{Host: ip, Port: 445, Share: mut(tg.share), Username: e.user, Password: e.pass})
		if c.Connect(context.Background()) == nil {
			_ = c.Disconnect(context.Background())
			return
		}
	}
	t.Skipf("SKIP(hardware_not_present): optional share %s is not mountable on any configured host", tg.share)
}

func TestLiveSMB_ListStatReadWalk(t *testing.T) {
	e := loadLiveEnv(t)
	for _, tg := range smbTargets() {
		tg := tg
		t.Run(tg.share, func(t *testing.T) {
			skipIfOptionalAbsent(t, e, tg)
			c := connectSMB(t, e, tg.share)
			ctx := context.Background()
			root, err := c.ListDirectory(ctx, ".")
			if err != nil {
				t.Fatalf("list root: %v", rd(err))
			}
			if len(root) == 0 {
				t.Fatalf("share %s root listing is empty", tg.share)
			}
			start := "."
			if tg.preferred != "" {
				fi, err := c.GetFileInfo(ctx, tg.preferred)
				if err != nil {
					t.Fatalf("stat %q (Cyrillic dir): %v", tg.preferred, rd(err))
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
				t.Fatalf("stat file: %v", rd(err))
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
				t.Fatalf("OpenSeekable: %v", rd(err))
			}
			defer sk.Close()
			if fi.Size > 8 {
				if _, err := sk.Seek(fi.Size-8, io.SeekStart); err != nil {
					t.Fatalf("seek: %v", rd(err))
				}
				tail := make([]byte, 8)
				if _, err := io.ReadFull(sk, tail); err != nil {
					t.Fatalf("read after seek to tail: %v", rd(err))
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
	for _, tg := range smbTargets() {
		tg := tg
		t.Run(tg.share, func(t *testing.T) {
			skipIfOptionalAbsent(t, e, tg)
			c := connectSMB(t, e, tg.share)
			ctx := context.Background()
			res := walk(t, c, ".", 2, 25)
			cyrNames, specNames := firstN(res.cyr, 12), firstN(res.special, 12)
			for _, p := range append(append([]string{}, cyrNames...), specNames...) {
				fi, err := c.GetFileInfo(ctx, p)
				if err != nil {
					t.Errorf("stat of listed name (non-ASCII/special) failed: %v", rd(err))
					continue
				}
				if fi.Name != pathpkg.Base(p) {
					t.Errorf("name round-trip mismatch: listed %q stat %q", pathpkg.Base(p), fi.Name)
				}
				ok, err := c.FileExists(ctx, p)
				if err != nil || !ok {
					t.Errorf("FileExists(listed name) = %v, %v", ok, rd(err))
				}
				if !utf8.ValidString(fi.Name) {
					t.Errorf("invalid UTF-8 after round trip")
				}
			}
			cyrTotal += len(cyrNames)
			specTotal += len(specNames)
			if specialRequired(tg.share) && len(specNames) == 0 {
				t.Errorf("share %s: 0 ASCII special-character names CHECKED (survey says they exist): the leg proved nothing", tg.share)
			}
			t.Logf("share=%s checked_cyrillic=%d checked_special=%d (seen cyrillic=%d special=%d)", tg.share, len(cyrNames), len(specNames), len(res.cyr), len(res.special))
		})
	}
	if cyrTotal == 0 {
		t.Errorf("0 Cyrillic names CHECKED on any share: the Cyrillic round-trip leg proved nothing")
	}
	if specTotal == 0 {
		t.Errorf("0 special-character names CHECKED on any share: the special-char leg proved nothing")
	}
}

func firstN(in []string, n int) []string {
	if len(in) > n {
		return in[:n]
	}
	return in
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
			t.Fatalf("FileExists(nonexistent) = %v, %v; want false, nil", ok, rd(err))
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

func ftpTargets() []ftpTarget {
	var out []ftpTarget
	for _, kv := range parseTargets("LIVE_FTP_SHARES", "DATA12:Филмови,DATA18:Серије,DATA20-3:Музика,WORK20:") {
		out = append(out, ftpTarget{kv[0], kv[1]})
	}
	return out
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
	for _, tg := range ftpTargets() {
		tg := tg
		t.Run(tg.share, func(t *testing.T) {
			c := connectFTP(t, e, tg)
			ctx := context.Background()
			root, err := c.ListDirectory(ctx, ".")
			if err != nil {
				t.Fatalf("list base: %v", rd(err))
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
					t.Errorf("GetFileInfo(dir %q) failed: %v", sd.Name, rd(err))
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
				t.Fatalf("GetFileInfo(file): %v", rd(err))
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
				t.Fatalf("control connection unusable after partial read: %v", rd(err))
			}
			if _, err := c.ListDirectory(ctx, "."); err != nil {
				t.Fatalf("list after partial read: %v", rd(err))
			}
			t.Logf("share=%s read_bytes=%d size_bytes=%d", tg.share, len(b), fi.Size)
		})
	}
}

func TestLiveFTP_NameRoundTrip(t *testing.T) {
	e := loadLiveEnv(t)
	cyrTotal, specTotal := 0, 0
	for _, tg := range ftpTargets() {
		tg := tg
		t.Run(tg.share, func(t *testing.T) {
			c := connectFTP(t, e, ftpTarget{tg.share, ""})
			ctx := context.Background()
			res := walk(t, c, ".", 2, 25)
			cyrNames, specNames := firstN(res.cyr, 12), firstN(res.special, 12)
			for _, p := range append(append([]string{}, cyrNames...), specNames...) {
				ok, err := c.FileExists(ctx, p)
				if err != nil || !ok {
					t.Errorf("FileExists(listed non-ASCII/special name) = %v, %v", ok, rd(err))
				}
				fi, err := c.GetFileInfo(ctx, p)
				if err != nil {
					t.Errorf("GetFileInfo(listed name) failed: %v", rd(err))
				} else if fi.Name != pathpkg.Base(p) {
					t.Errorf("name round-trip mismatch: listed %q stat %q", pathpkg.Base(p), fi.Name)
				}
			}
			cyrTotal += len(cyrNames)
			specTotal += len(specNames)
			if specialRequired(tg.share) && len(specNames) == 0 {
				t.Errorf("FTP share %s: 0 ASCII special-character names CHECKED (survey says they exist)", tg.share)
			}
			t.Logf("share=%s checked_cyrillic=%d checked_special=%d (seen cyrillic=%d special=%d)", tg.share, len(cyrNames), len(specNames), len(res.cyr), len(res.special))
		})
	}
	if cyrTotal == 0 {
		t.Errorf("0 Cyrillic names CHECKED on any FTP share")
	}
	if specTotal == 0 {
		t.Errorf("0 special-character names CHECKED on any FTP share")
	}
	// a Cyrillic base Path (as the product configures it) must be accepted and listable
	c := connectFTP(t, e, ftpTarget{"DATA12", "Филмови"})
	if ents, err := c.ListDirectory(context.Background(), "."); err != nil || len(ents) == 0 {
		t.Fatalf("Cyrillic base path not listable: n=%d err=%v", len(ents), rd(err))
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
			t.Fatalf("ListDirectory(nonexistent) returned no error")
		}
		if _, err := c.ReadFile(ctx, "no-such-dir-live-xyz/a.bin"); err == nil {
			t.Fatalf("read of nonexistent file succeeded")
		}
		ok, err := c.FileExists(ctx, "no-such-dir-live-xyz")
		if err != nil || ok {
			t.Fatalf("FileExists(nonexistent) = %v, %v; want false, nil", ok, rd(err))
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

// Close semantics and lifecycle against the real FTP service.
func TestLiveFTP_PartialCloseAndDisconnectSemantics(t *testing.T) {
	e := loadLiveEnv(t)
	ctx := context.Background()
	tg := ftpTargets()[0]
	c := connectFTP(t, e, tg)
	res := walk(t, c, ".", 3, 40)
	if res.firstFile == "" {
		t.Skip("SKIP(no_file_in_bound): no file found for the FTP close-semantics test")
	}
	fi, err := c.GetFileInfo(ctx, res.firstFile)
	if err != nil || fi.Size < 4096 {
		t.Skipf("SKIP(no_file_in_bound): need a file of at least 4096 bytes (err=%v)", rd(err))
	}
	// 1) deliberate partial read: Close must be nil and the SAME client must keep working
	rc, err := c.ReadFile(ctx, res.firstFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", rd(err))
	}
	buf := make([]byte, 100)
	if _, err := io.ReadFull(rc, buf); err != nil {
		t.Fatalf("read: %v", rd(err))
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close after partial read = %v, want nil", rd(err))
	}
	if !c.IsConnected() {
		t.Fatalf("IsConnected false after partial-read Close")
	}
	if _, err := c.ListDirectory(ctx, "."); err != nil {
		t.Fatalf("client unusable after partial-read Close: %v", rd(err))
	}
	// 2) ModTime of a file is a real time, not "now"
	fi2, err := c.GetFileInfo(ctx, res.firstFile)
	if err != nil {
		t.Fatalf("GetFileInfo: %v", rd(err))
	}
	if fi2.ModTime.IsZero() {
		t.Logf("UNCONFIRMED: server gave no mtime (MDTM and listing time both empty)")
	} else if d := time.Since(fi2.ModTime); d < time.Minute && d > -time.Minute {
		t.Errorf("file ModTime is within a minute of now (%s ago): looks fabricated", d)
	} else {
		t.Logf("file mtime age_days=%d via MDTM-or-listing", int(d.Hours()/24))
	}
	// 3) Disconnect, then Close of an outstanding reader must not resurrect the client
	rc2, err := c.ReadFile(ctx, res.firstFile)
	if err != nil {
		t.Fatalf("ReadFile 2: %v", rd(err))
	}
	if _, err := io.ReadFull(rc2, buf); err != nil {
		t.Fatalf("read 2: %v", rd(err))
	}
	_ = c.Disconnect(ctx)
	_ = rc2.Close()
	if c.IsConnected() {
		t.Fatalf("IsConnected true after Disconnect + reader Close: Close redialled")
	}
	if _, err := c.ListDirectory(ctx, "."); err == nil {
		t.Fatalf("ListDirectory succeeded on a disconnected client")
	}
}
