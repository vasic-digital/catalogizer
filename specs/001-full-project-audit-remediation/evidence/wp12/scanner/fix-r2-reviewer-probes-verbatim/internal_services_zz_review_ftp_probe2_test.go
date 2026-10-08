package services

// WF22 review probe R18: a RELATIVE FTP root path (now forwarded by PA-01) is applied twice: Connect does CWD <path>, then resolvePath prefixes it
// again. Confirms the author's UNCONFIRMED note. Uses a small RFC 959/3659 server with CWD + relative path resolution.

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"path"
	"strings"
	"testing"

	"catalogizer/filesystem"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startCwdServer(t *testing.T, tree map[string][]string, seen *[]string) (string, int) {
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
				cwd := "/"
				var dl net.Listener
				abs := func(p string) string {
					if strings.HasPrefix(p, "/") {
						return path.Clean(p)
					}
					return path.Clean(path.Join(cwd, p))
				}
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
					case "CWD":
						if _, ok := tree[abs(arg)]; ok {
							cwd = abs(arg)
							say("250 ok")
						} else {
							say("550 no")
						}
					case "EPSV":
						dl, _ = net.Listen("tcp", "127.0.0.1:0")
						say(fmt.Sprintf("229 Entering Extended Passive Mode (|||%d|)", dl.Addr().(*net.TCPAddr).Port))
					case "MLSD":
						*seen = append(*seen, arg+" => "+abs(arg))
						ents, ok := tree[abs(arg)]
						if !ok || dl == nil {
							say("550 no such directory")
							continue
						}
						say("150 here")
						if dc, err := dl.Accept(); err == nil {
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

func TestReviewProbe_R18_FTPRelativeRootPathAppliedTwice(t *testing.T) {
	m := "modify=20260102030405"
	tree := map[string][]string{
		"/":       {"type=dir;" + m + "; movies"},
		"/movies": {"type=file;size=3;" + m + "; a.mkv"},
	}
	run := func(root string) ([]string, []string, error) {
		var seen []string
		host, port := startCwdServer(t, tree, &seen)
		c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{Protocol: "ftp",
			Settings: map[string]interface{}{"host": host, "port": port, "username": "u", "password": "p", "path": root}})
		require.NoError(t, err)
		require.NoError(t, c.Connect(context.Background()))
		defer c.Disconnect(context.Background())
		rec := newRecorder()
		err = NewFTPScanner(nil).WithRecorder(rec.record).ScanPath(context.Background(), c, jobOf("/", 10, "full"), &ScanStatus{})
		return rec.sorted(), seen, err
	}
	got, seen, err := run("/movies")
	t.Logf("R18 absolute /movies: recorded=%v listings=%v err=%v", got, seen, err)
	require.NoError(t, err, "control: the absolute form works")
	require.Equal(t, []string{"/a.mkv"}, got)
	got, seen, err = run("movies")
	t.Logf("R18 relative movies: recorded=%v listings=%v err=%v", got, seen, err)
	assert.NoError(t, err, "a relative FTP root path must scan the same tree as its absolute form")
	assert.Equal(t, []string{"/a.mkv"}, got)
}
