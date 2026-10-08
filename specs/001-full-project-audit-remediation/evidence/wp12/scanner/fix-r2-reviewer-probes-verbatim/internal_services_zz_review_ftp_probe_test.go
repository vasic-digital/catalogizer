package services

// WF22 review probe R16: an MLSD-capable FTP server (pure-ftpd, ProFTPD/Synology) returns the "." (cdir) and ".." (pdir) entries; jlaffaye/ftp
// passes them through, the catalog-api FTP client does not filter them, and the generic scanner counts each as an ERROR. A completely clean
// scan therefore reports error_count = 2 x directories. Minimal in-process RFC 3659 server (review probe, scratch copy only).

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"catalogizer/filesystem"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startMLSDServer(t *testing.T, tree map[string][]string) (string, int) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				w := bufio.NewWriter(c)
				r := bufio.NewReader(c)
				say := func(s string) { _, _ = w.WriteString(s + "\r\n"); _ = w.Flush() }
				say("220 probe")
				var dl net.Listener
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimRight(line, "\r\n")
					cmd, arg, _ := strings.Cut(line, " ")
					switch strings.ToUpper(cmd) {
					case "USER":
						say("331 pass")
					case "PASS":
						say("230 ok")
					case "FEAT":
						say("211-Features:\r\n MLST type*;size*;modify*;\r\n EPSV\r\n211 End")
					case "TYPE", "NOOP":
						say("200 ok")
					case "EPSV":
						dl, _ = net.Listen("tcp", "127.0.0.1:0")
						say(fmt.Sprintf("229 Entering Extended Passive Mode (|||%d|)", dl.Addr().(*net.TCPAddr).Port))
					case "MLSD":
						ents, ok := tree[arg]
						if !ok || dl == nil {
							say("550 no such directory")
							continue
						}
						say("150 here")
						dc, err := dl.Accept()
						if err == nil {
							for _, e := range ents {
								_, _ = dc.Write([]byte(e + "\r\n"))
							}
							dc.Close()
						}
						dl.Close()
						dl = nil
						say("226 done")
					case "QUIT":
						say("221 bye")
						return
					default:
						say("502 not implemented")
					}
				}
			}(c)
		}
	}()
	return "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
}

func TestReviewProbe_R16_MLSDDotEntriesCountedAsErrors(t *testing.T) {
	m := "modify=20260102030405"
	tree := map[string][]string{
		"/":      {"type=cdir;" + m + "; .", "type=pdir;" + m + "; ..", "type=dir;" + m + "; music", "type=file;size=3;" + m + "; a.mkv"},
		"/music": {"type=cdir;" + m + "; .", "type=pdir;" + m + "; ..", "type=file;size=3;" + m + "; b.flac"},
	}
	host, port := startMLSDServer(t, tree)
	c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{Protocol: "ftp",
		Settings: map[string]interface{}{"host": host, "port": port, "username": "u", "password": "p"}})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	defer c.Disconnect(context.Background())
	rec := newRecorder()
	st := &ScanStatus{}
	require.NoError(t, NewFTPScanner(nil).WithRecorder(rec.record).ScanPath(context.Background(), c, jobOf("/", 10, "full"), st))
	t.Logf("R16 recorded=%v errorCount=%d", rec.sorted(), st.GetSnapshot().ErrorCount)
	require.ElementsMatch(t, []string{"/a.mkv", "/music", "/music/b.flac"}, rec.sorted(), "control: the real FTP client + scanner read the tree")
	assert.Equal(t, int64(0), st.GetSnapshot().ErrorCount, "a clean scan of a clean tree must report zero errors")
}
