//go:build nasleg

package nasclients

// Diagnostic of the one client discrepancy of the main run: Synology6 / DATA20-3, the FTP/FTPS client fails a directory listing at depth 2 with
// "directory listing has lines that could not be parsed" while the SFTP client lists the same directory. This test (NCL_DIAG=1) re-walks the same deterministic
// sample over plain FTP until that error appears, records the STRUCTURE of the unparsed line (never the name), then lists the same directory over SFTP
// and records which peculiarities its entry names have (counts only). Read-only, same chain and budget as the main run.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"digital.vasic.filesystem/pkg/client"
	"digital.vasic.filesystem/pkg/ftp"
	"digital.vasic.filesystem/pkg/sftp"
)

func nameClasses(names []string) map[string]int {
	m := map[string]int{"entries": len(names)}
	for _, n := range names {
		if strings.ContainsAny(n, "\r\n") {
			m["name_has_CR_or_LF"]++
		}
		for _, r := range n {
			if r < 0x20 || r == 0x7f {
				m["name_has_control_char"]++
				break
			}
		}
		if strings.HasPrefix(n, " ") {
			m["name_leading_space"]++
		}
		if strings.HasSuffix(n, " ") {
			m["name_trailing_space"]++
		}
		if strings.Contains(n, "; ") {
			m["name_contains_semicolon_space"]++
		}
		if strings.Contains(n, "=") {
			m["name_contains_equals"]++
		}
		if !utf8.ValidString(n) {
			m["name_invalid_utf8"]++
		}
		for _, r := range n {
			if r > 0x7f {
				m["name_non_ascii"]++
				break
			}
		}
		if len(n) > 200 {
			m["name_len_gt_200_bytes"]++
		}
	}
	return m
}

func TestDiagFTPUnparsed(t *testing.T) {
	if os.Getenv("NCL_DIAG") != "1" {
		t.Skip("NCL_DIAG not set")
	}
	n := 6
	host := hostFor(n)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	res := map[string]interface{}{"schema": "wp12-nas-client-diag/1", "alias": fmt.Sprintf("Synology%d", n), "share": "DATA20-3", "names_recorded": false}
	defer func() {
		b, _ := json.MarshalIndent(res, "", "  ")
		s := string(b)
		if strings.Contains(s, os.Getenv("NCL_PW")) || strings.Contains(s, host) {
			t.Errorf("LEAK in diag result")
			return
		}
		_ = os.WriteFile(os.Getenv("NCL_OUT")+"/diag-ftp-6.json", b, 0o644)
	}()
	raw := ftp.NewFTPClient(&ftp.Config{Host: host, Port: 21, Username: os.Getenv("NCL_USER"), CredentialRef: "nas", Path: "/", Resolver: ftpRes{}, TLSMode: ftp.TLSNone, TrustedLAN: true,
		DialTimeout: 15 * time.Second, IOTimeout: 30 * time.Second})
	b := newBudget()
	c, sp, err := buildChain(raw, b)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Connect(ctx); err != nil {
		res["connect_error"] = scrub(err.Error())
		return
	}
	defer c.Disconnect(context.Background())
	type item struct {
		p string
		d int
	}
	q := []item{{"/DATA20-3", 0}}
	reqs := 0
	for len(q) > 0 && reqs < maxListPerShare {
		it := q[0]
		q = q[1:]
		reqs++
		lst, err := c.ListDirectory(ctx, it.p)
		if err != nil {
			res["failing_dir_depth"] = it.d
			res["ftp_error_type"] = fmt.Sprintf("%T", err)
			if m := reQuoted.FindAllString(err.Error(), -1); len(m) > 0 {
				line := m[len(m)-1]
				sig := map[string]interface{}{"quoted_len": len(line), "valid_utf8": utf8.ValidString(line)}
				body := strings.Trim(line, "\"")
				sig["starts_with_type_fact"] = strings.HasPrefix(body, "type=")
				sig["has_semicolon_space_separator"] = strings.Contains(body, "; ")
				sig["semicolons"] = strings.Count(body, ";")
				na := 0
				for _, r := range body {
					if r > 0x7f {
						na++
					}
				}
				sig["non_ascii_runes"] = na
				if i := strings.Index(body, "; "); i >= 0 {
					facts := body[:i]
					var keys []string
					for _, f := range strings.Split(facts, ";") {
						if k, _, ok := strings.Cut(f, "="); ok {
							keys = append(keys, strings.ToLower(k))
						} else if f != "" {
							keys = append(keys, "(fact-without-value)")
						}
					}
					sig["fact_keys"] = keys
					sig["name_part_len"] = len(body) - i - 2
				}
				res["unparsed_line_signature"] = sig
			}
			res["error_message_redacted"] = redactMsg(err, it.p)
			// the same directory over SFTP
			fps := []string{}
			if m := loadJSON(fmt.Sprintf("%s/sftp-%d.json", os.Getenv("NCL_REF"), n)); m != nil {
				if arr, ok := m["host_keys"].([]interface{}); ok {
					for _, a := range arr {
						if f, _ := a.(map[string]interface{})["fingerprint_sha256"].(string); f != "" {
							fps = append(fps, f)
						}
					}
				}
			}
			store := sftp.NewMemPinStore()
			for _, fp := range fps {
				if _, perr := sftp.Pin(ctx, store, host, 22, sftp.Confirmation{Owner: confirmedBy, Fingerprint: fp}); perr == nil {
					break
				}
				time.Sleep(time.Second)
			}
			sc := sftp.NewSFTPClient(&sftp.Config{Host: host, Port: 22, Username: os.Getenv("NCL_USER"), CredentialRef: "nas", Root: "/", Resolver: sftpRes{}, PinStore: store, DialTimeout: 20 * time.Second})
			time.Sleep(time.Second)
			if cerr := sc.Connect(ctx); cerr != nil {
				res["sftp_connect_error"] = scrub(cerr.Error())
				return
			}
			defer sc.Disconnect(ctx)
			sl, lerr := sc.ListDirectory(ctx, it.p)
			if lerr != nil {
				res["sftp_list_error"] = redactMsg(lerr, it.p)
				return
			}
			var names []string
			for _, f := range sl {
				if f != nil {
					names = append(names, f.Name)
				}
			}
			res["sftp_entry_name_classes"] = nameClasses(names)
			res["sftp_entries_total"] = len(names)
			return
		}
		es := make([]*client.FileInfo, 0, len(lst))
		for _, f := range lst {
			if f != nil && f.IsDir && f.Name != "." && f.Name != ".." && f.Name != "" {
				es = append(es, f)
			}
		}
		sort.Slice(es, func(i, j int) bool { return es[i].Name < es[j].Name })
		// the main walk enqueues sorted directories of the sorted listing; the order of dirs is the same here
		if it.d+1 < maxDepth {
			for _, f := range es {
				q = append(q, item{path.Join(it.p, f.Name), it.d + 1})
			}
		}
	}
	_ = sp
	res["no_failure_reproduced_within_requests"] = reqs
}
