package tests

import (
	"os"
	"path/filepath"
	"testing"
)

// writeRunEnv writes a per-run env file of the shape scripts/test-infra/gen_env.sh produces (fixture values, not credentials).
func writeRunEnv(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestInfraHelperReadsThePerRunEnvFile: the helper derives addresses and credentials from the env file of the per-run stack, not from fixed ports or literal defaults (WF12 F1).
func TestInfraHelperReadsThePerRunEnvFile(t *testing.T) {
	t.Setenv(EnvFileVariable, writeRunEnv(t, "TI_PROJECT=catalogizer-test-x\nTI_PORT_FTP=40021\nTI_PORT_SMB=40445\nTI_PORT_WEBDAV=40080\nTI_FTP_USER=fu\nTI_FTP_PASSWORD=fp\nTI_SMB_USER=su\nTI_SMB_PASSWORD=sp\nTI_WEBDAV_USER=wu\nTI_WEBDAV_PASSWORD=wp\n"))
	for _, k := range []string{"FTP_TEST_SERVER", "SMB_TEST_SERVER", "WEBDAV_TEST_SERVER", "NFS_TEST_SERVER", "FTP_TEST_USER", "FTP_TEST_PASS", "SMB_TEST_USER", "SMB_TEST_PASS", "WEBDAV_TEST_USER", "WEBDAV_TEST_PASS"} {
		t.Setenv(k, "")
	}
	if got := ftpAddr(); got != "127.0.0.1:40021" {
		t.Errorf("ftpAddr() = %q", got)
	}
	if got := smbAddr(); got != "127.0.0.1:40445" {
		t.Errorf("smbAddr() = %q", got)
	}
	if got := webdavAddr(); got != "127.0.0.1:40080" {
		t.Errorf("webdavAddr() = %q", got)
	}
	if got := nfsAddr(); got != "" {
		t.Errorf("nfsAddr() = %q, want empty (the user-space NFS server publishes no host port)", got)
	}
	if c := FTPCredentials(); c.Username != "fu" || c.Password != "fp" {
		t.Errorf("FTPCredentials() = %+v", c)
	}
	if c := SMBCredentials(); c.Username != "su" || c.Password != "sp" {
		t.Errorf("SMBCredentials() = %+v", c)
	}
	if c := WebDAVCredentials(); c.Username != "wu" || c.Password != "wp" {
		t.Errorf("WebDAVCredentials() = %+v", c)
	}
}

// TestInfraHelperExplicitOverrideWinsAndNoStackMeansNotConfigured: an explicit server variable beats the env file; with neither there is no fixed fallback.
func TestInfraHelperExplicitOverrideWinsAndNoStackMeansNotConfigured(t *testing.T) {
	t.Setenv(EnvFileVariable, writeRunEnv(t, "TI_PORT_FTP=40021\n"))
	t.Setenv("FTP_TEST_SERVER", "10.0.0.9:2121")
	if got := ftpAddr(); got != "10.0.0.9:2121" {
		t.Errorf("explicit override not honoured: %q", got)
	}
	t.Setenv(EnvFileVariable, "")
	t.Setenv("FTP_TEST_SERVER", "")
	t.Setenv("FTP_TEST_USER", "")
	if ftpAddr() != "" || FTPCredentials().Username != "" || FTPCredentials().Password != "" {
		t.Errorf("no stack configured must yield empty values, got addr %q creds %+v", ftpAddr(), FTPCredentials())
	}
	if isReachable("") {
		t.Error("an empty address must not count as reachable")
	}
	t.Setenv(EnvFileVariable, filepath.Join(t.TempDir(), "does-not-exist"))
	if ftpAddr() != "" {
		t.Error("an unreadable env file must yield not-configured")
	}
}
