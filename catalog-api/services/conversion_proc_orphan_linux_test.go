//go:build linux

package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// F7 (scaling-suite finding): converter children get their own process group (Setpgid) but, without
// Pdeathsig, outlive the server when it dies abruptly (SIGKILL, OOM kill): they keep running,
// writing output nobody tracks, until they finish on their own. REAL processes, no double: the test
// re-executes its own binary as the "server", which starts a real long-running child through the
// production setProcessGroup, then SIGKILLs that server and observes the child.
const orphanHelperEnv = "CATALOGIZER_ORPHAN_HELPER"

// init (not TestMain: the package already has one) switches the re-executed test binary into the helper role.
func init() {
	if os.Getenv(orphanHelperEnv) == "1" {
		// "server" role: start a real converter-like child through the production helper, report its pid, wait.
		cmd := exec.CommandContext(context.Background(), "sleep", "300")
		setProcessGroup(cmd)
		if err := cmd.Start(); err != nil {
			fmt.Println("START-ERROR", err)
			os.Exit(3)
		}
		fmt.Println("CHILD", cmd.Process.Pid)
		_ = cmd.Wait()
		os.Exit(0)
	}
}

func procRunning(pid int) bool {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	// state is the field after the ")" closing the comm; a zombie (Z) is dead for our purpose
	s := string(b)
	i := strings.LastIndex(s, ")")
	if i < 0 || i+2 >= len(s) {
		return false
	}
	return s[i+2] != 'Z' && s[i+2] != 'X'
}

func TestConverterChildDiesWhenServerIsSIGKILLed(t *testing.T) {
	srv := exec.Command(os.Args[0], "-test.run=NONE")
	srv.Env = append(os.Environ(), orphanHelperEnv+"=1")
	out, err := srv.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, srv.Start())
	t.Cleanup(func() { _ = srv.Process.Kill(); _, _ = srv.Process.Wait() })

	line, err := bufio.NewReader(out).ReadString('\n')
	require.NoError(t, err)
	fields := strings.Fields(line)
	require.Len(t, fields, 2, "helper said: %q", line)
	require.Equal(t, "CHILD", fields[0], line)
	child, err := strconv.Atoi(fields[1])
	require.NoError(t, err)
	t.Cleanup(func() { // never leave the long sleeper behind, whatever the verdict
		if child > 1 && procRunning(child) {
			_ = syscall.Kill(child, syscall.SIGKILL)
		}
	})
	require.True(t, procRunning(child), "child must be running before the server is killed")

	require.NoError(t, srv.Process.Signal(syscall.SIGKILL))
	_, _ = srv.Process.Wait()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !procRunning(child) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("orphaned converter child %d still running 5s after its server was SIGKILLed (no Pdeathsig)", child)
}
