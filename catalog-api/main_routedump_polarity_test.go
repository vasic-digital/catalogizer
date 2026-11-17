package main

// G1FIXFU4: reviewer mutants R1/R1c/R1d (negated and combined build-constraint polarities) adopted as named tests. The scanner must
// treat a file as possibly compiled in the default build when SOME assignment of the tags it uses (routedump off) satisfies its
// constraint, not only when "every other tag is true" does. Unit-class source scan (§11.4.289(C)).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeRaw(t *testing.T, dir, name, src string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600))
}

func TestAuthSurfaceDumpIsolation_PolarityMutants(t *testing.T) {
	const stripInit = "func init() {\n\tgo func() {\n\t\tfor len(rvSinkEngine.Handlers) > 0 {\n\t\t\trvSinkEngine.Handlers = nil\n\t\t}\n\t}()\n}\n"
	cases := []struct {
		name string
		want []string
		fn   func(d string)
	}{
		{"R1_off_file_windows_plus_new_notwindows_hook_file", []string{"exactly one routeDumpHook (found 2)", "must live in main_routedump_off.go", "body must be empty"}, func(d string) {
			edit(t, d, "main_routedump_off.go", "//go:build !routedump", "//go:build !routedump && windows")
			writeRaw(t, d, "main_r1.go", "//go:build !routedump && !windows\n\npackage main\n\nimport \"github.com/gin-gonic/gin\"\n\nvar rvSink *gin.Engine\nvar rvSinkEngine *gin.Engine\n\nfunc routeDumpHook(e *gin.Engine) { rvSink = e; rvSinkEngine = e }\n\n"+stripInit)
		}},
		{"R1c_hook_body_writes_engine_fields", []string{"body must be empty"}, func(d string) {
			edit(t, d, "main_routedump_off.go", "//go:build !routedump", "//go:build !routedump && windows")
			writeRaw(t, d, "main_r1c.go", "//go:build !routedump && !windows\n\npackage main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc routeDumpHook(e *gin.Engine) {\n\te.TrustedPlatform = \"x\"\n\te.ForwardedByClientIP = true\n\te.MaxMultipartMemory = 1\n\te.ContextWithFallback = true\n}\n")
		}},
		{"R1d_dump_file_routedump_or_notwindows", []string{"does not exclude it from the default build"}, func(d string) {
			edit(t, d, "main_routechains.go", "//go:build routedump\n", "//go:build routedump || !windows\n")
		}},
		{"R1e_dump_machinery_under_negated_only", []string{"does not exclude it from the default build"}, func(d string) {
			writeRaw(t, d, "main_r1e.go", "//go:build !routedump && !windows\n\npackage main\n\ntype routeChain struct{}\n")
		}},
		{"R1f_filename_suffix_dump_machinery", []string{"does not exclude it from the default build"}, func(d string) {
			writeRaw(t, d, "main_r1f_linux.go", "package main\n\ntype routeHandler struct{}\n")
		}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(mutate(t, c.fn), "\n")
			for _, w := range c.want {
				require.Contains(t, got, w)
			}
		})
	}
	// Golden-false: a dump file whose constraint can never hold with routedump off stays accepted in every polarity.
	require.Empty(t, mutate(t, func(d string) {
		edit(t, d, "main_routechains.go", "//go:build routedump\n", "//go:build routedump && !windows\n")
	}))
	require.Empty(t, mutate(t, func(d string) {
		writeRaw(t, d, "main_r1g_windows.go", "//go:build routedump\n\npackage main\n\ntype routeChain struct{}\n")
	}))
}
