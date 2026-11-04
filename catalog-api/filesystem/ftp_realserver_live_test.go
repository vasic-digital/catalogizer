package filesystem

// LIVE-CLASS TEST (constitution §11.4.289/§11.4.292): the server here is a REAL, independent FTP server process (pyftpdlib, started as a
// separate python3 process on a loopback port, serving a real directory); the client under test is the product's FTPClient, unmodified.
// The test is skipped (honest SKIP, never a stand-in) when python3 or pyftpdlib is not available: set FTP_PYFTPDLIB_DIR to a directory
// that contains the pyftpdlib package (it is used as PYTHONPATH, so it may be a path list; Python >= 3.12 additionally needs the pyasynchat/pyasyncore backports on it).

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const pyFTPServer = `
import sys
from pyftpdlib.authorizers import DummyAuthorizer
from pyftpdlib.handlers import FTPHandler
from pyftpdlib.servers import FTPServer
root = sys.argv[1]
a = DummyAuthorizer()
a.add_user("tester", "t3st-only", root, perm="elradfmwMT")
h = FTPHandler
h.authorizer = a
h.passive_ports = range(52000, 52400)
s = FTPServer(("127.0.0.1", 0), h)
print(s.address[1], flush=True)
s.serve_forever()
`

// startRealFTPServer starts pyftpdlib serving root and returns its port.
func startRealFTPServer(t *testing.T, root string) int {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available: real FTP server cannot be started (honest SKIP)")
	}
	libDir := os.Getenv("FTP_PYFTPDLIB_DIR")
	probe := exec.Command(py, "-c", "import pyftpdlib")
	probe.Env = append(os.Environ(), "PYTHONPATH="+libDir)
	if err := probe.Run(); err != nil {
		t.Skip("pyftpdlib not importable (set FTP_PYFTPDLIB_DIR): real FTP server cannot be started (honest SKIP)")
	}
	cmd := exec.Command(py, "-c", pyFTPServer, root)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+libDir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatalf("real FTP server did not report its port: %v", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("bad port line %q", line)
	}
	return port
}

func realFTPFixture(t *testing.T, nfiles int) (*FTPClient, map[string]int64) {
	t.Helper()
	root := t.TempDir()
	sizes := map[string]int64{}
	for i := 0; i < nfiles; i++ {
		name := fmt.Sprintf("f%02d.bin", i)
		n := 100 + i*7
		if err := os.WriteFile(filepath.Join(root, name), make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
		sizes[name] = int64(n)
	}
	port := startRealFTPServer(t, root)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "tester", Password: "t3st-only", Path: "/"})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect to real FTP server: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	return c, sizes
}

// ST-D2: one FTPClient shared by 4 goroutines must give correct results (and never hang).
func TestLiveRealFTP_SharedClientParallelStat(t *testing.T) {
	c, sizes := realFTPFixture(t, 16)
	done := make(chan []string, 1)
	go func() {
		var mu sync.Mutex
		var bad []string
		var wg sync.WaitGroup
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				ctx := context.Background()
				for r := 0; r < 20; r++ {
					for name, want := range sizes {
						fi, err := c.GetFileInfo(ctx, "/"+name)
						if err != nil || fi.Size != want {
							mu.Lock()
							bad = append(bad, fmt.Sprintf("g%d stat %s: size=%v err=%v want %d", g, name, fi, err, want))
							mu.Unlock()
							return
						}
					}
					ok, err := c.FileExists(ctx, "/nope-"+strconv.Itoa(g))
					if err != nil || ok {
						mu.Lock()
						bad = append(bad, fmt.Sprintf("g%d exists(nonexistent)=%v err=%v", g, ok, err))
						mu.Unlock()
						return
					}
				}
			}(g)
		}
		wg.Wait()
		done <- bad
	}()
	select {
	case bad := <-done:
		if len(bad) > 0 {
			t.Fatalf("%d goroutine(s) saw wrong results, first: %s", len(bad), bad[0])
		}
	case <-time.After(60 * time.Second):
		t.Fatal("HANG: parallel operations on one FTPClient did not finish in 60s")
	}
}

// ST-D2 (deadline half): a server that accepts, greets, then goes silent must not block an operation past its context.
func TestFTPOperationHonoursContextDeadlineOnSilentServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				r := bufio.NewReader(conn)
				fmt.Fprint(conn, "220 ready\r\n")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(cmd, "USER"):
						fmt.Fprint(conn, "331 pw\r\n")
					case strings.HasPrefix(cmd, "PASS"):
						fmt.Fprint(conn, "230 ok\r\n")
					case strings.HasPrefix(cmd, "FEAT"):
						fmt.Fprint(conn, "211 end\r\n")
					case strings.HasPrefix(cmd, "OPTS"):
						fmt.Fprint(conn, "200 ok\r\n")
					case strings.HasPrefix(cmd, "TYPE"):
						fmt.Fprint(conn, "200 ok\r\n")
					default:
						// every other command (SIZE/MDTM/MLST/...) is never answered
					}
				}
			}(conn)
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	res := make(chan error, 1)
	start := time.Now()
	go func() { _, err := c.GetFileInfo(ctx, "/x"); res <- err }()
	select {
	case err := <-res:
		if err == nil {
			t.Fatal("expected an error from a silent server")
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Fatalf("returned after %v, not near the 700ms context deadline", d)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("HANG: operation ignored its context deadline on a silent server")
	}
}
