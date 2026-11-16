package errleak

import "testing"

// ELFUR-m1: isStdStream and isSentinel are name heuristics. They were substring/prefix matches, so a response
// writer or error value that merely CONTAINS stdout/stderr, or starts with "Err" followed by lowercase, was
// exempted. A differential over the real catalog-api tree (13 sites before and after) showed no real site
// depends on the loose forms, so they are tightened to the exact spellings the tree really uses. errName
// (suffix "err") is deliberately NOT tightened: 158 distinct real names depend on it (e.g. loginErr, rerr).
func TestNameHeuristicsStayTight(t *testing.T) {
	cases := map[string]struct {
		body string
		want int
	}{
		// golden-bad: must be flagged
		"stderr_substring_is_http_writer":   {"\tfmt.Fprintf(stderrProxy, \"%s\", err.Error())\n", 1},
		"stdout_field_is_http_writer":       {"\tfmt.Fprint(resp.Stdout, err.Error())\n", 1},
		"stdoutWriter_name_not_stream":      {"\tfmt.Fprintln(stdoutWriter, err)\n", 1},
		"Err_prefix_lowercase_not_sentinel": {"\tc.JSON(500, gin.H{\"e\": res.ErrorBody.Error()})\n", 1},
		"Errored_not_sentinel":              {"\tc.JSON(500, gin.H{\"e\": res.Errored.Error()})\n", 1},
		// golden-good: must stay clean
		"os_Stderr":     {"\tfmt.Fprintf(os.Stderr, \"%v\", err)\n", 0},
		"os_Stdout":     {"\tfmt.Fprintln(os.Stdout, err)\n", 0},
		"local_stderr":  {"\tfmt.Fprintf(stderr, \"%v\", err)\n", 0},
		"local_stdout":  {"\tfmt.Fprintf(stdout, \"%v\", err)\n", 0},
		"pkg_sentinel":  {"\tc.JSON(400, gin.H{\"e\": services.ErrSmartLimit.Error()})\n", 0},
		"bare_sentinel": {"\tc.JSON(400, gin.H{\"e\": ErrNotFound.Error()})\n", 0},
	}
	for name, tc := range cases {
		if got := hits(t, fnSrc("", tc.body)); len(got) != tc.want {
			t.Errorf("%s: want %d hit(s), got %v", name, tc.want, got)
		}
	}
}
