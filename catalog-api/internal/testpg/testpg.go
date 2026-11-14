// Package testpg starts a throwaway PostgreSQL 15 container (rootless podman,
// random loopback port) for integration tests that need a REAL PostgreSQL.
// It is test support only; production code must not import it.
package testpg

import (
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Image is the pinned test image.
// Start returns a DSN and container name. It SKIPs (never substitutes a fake)
// when podman or the image is unavailable.
const Image = "docker.io/library/postgres:15-alpine"

func Start(t testing.TB) (dsn, container string) {
	t.Helper()
	return StartImage(t, Image)
}

// ImageGlibc is a Debian-based PostgreSQL 15 image whose default database collation is glibc en_US.utf8 (Image is the musl/alpine
// one, byte-order). A test whose result may depend on collation runs against both.
const ImageGlibc = "docker.io/library/postgres:15"

// StartImage is Start for an explicit image; it SKIPs when podman or that image is unavailable locally (nothing is pulled).
func StartImage(t testing.TB, img string) (dsn, container string) {
	t.Helper()
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("SKIP hardware_not_present: podman not installed")
	}
	if err := exec.Command("podman", "image", "exists", img).Run(); err != nil {
		t.Skipf("SKIP hardware_not_present: image %s not present locally", img)
	}
	name := fmt.Sprintf("catalogizer-migtest-%d", time.Now().UnixNano())
	out, err := exec.Command("podman", "run", "-d", "--rm", "--name", name,
		"-e", "POSTGRES_PASSWORD=migtest", "-e", "POSTGRES_DB=migtest",
		"-p", "127.0.0.1::5432", img).CombinedOutput()
	if err != nil {
		t.Fatalf("podman run: %v: %s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("podman", "rm", "-f", name).Run() })

	portOut, err := exec.Command("podman", "port", name, "5432/tcp").Output()
	if err != nil {
		t.Fatalf("podman port: %v", err)
	}
	hostPort := strings.TrimSpace(string(portOut))
	hostPort = hostPort[strings.LastIndex(hostPort, ":")+1:]
	dsn = fmt.Sprintf("postgres://postgres:migtest@127.0.0.1:%s/migtest?sslmode=disable", hostPort)

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		// The official image restarts the server once after init; require
		// a successful query AND the final "ready to accept connections" twice.
		logs, _ := exec.Command("podman", "logs", name).CombinedOutput()
		if strings.Count(string(logs), "ready to accept connections") >= 2 {
			if sdb, err := sql.Open("postgres", dsn); err == nil {
				if sdb.Ping() == nil {
					sdb.Close()
					return dsn, name
				}
				sdb.Close()
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("postgres container did not become ready")
	return "", ""
}
