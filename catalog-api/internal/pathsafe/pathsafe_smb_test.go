package pathsafe

import "testing"

// SMB-FU-10: plain-share path checks beyond "..": alternate data streams (':') and Windows device names.
func TestSafeRemoteSharePath(t *testing.T) {
	bad := []string{
		"a/../b", `a\..\b`, "..", "x\x00", // inherited from SafeRemotePath
		"a:b", "file.txt:stream", "dir/file:$DATA", `dir\f:s`, ":", "C:",
		"CON", "con", "Nul", "PRN", "AUX", "COM1", "com9", "LPT1", "lpt9",
		"CON.txt", "nul.tar.gz", "dir/AUX", `dir\COM3.log`, "ok/LPT5.x/more",
		"COM0", "LPT0", "com0.txt", "COM¹", "COM²", "COM³", "LPT¹", "LPT²", "LPT³", "COM¹.txt", "dir/lpt³.log",
		"CONIN$", "CONOUT$", "CLOCK$", "conin$.txt", "..  ", "... ", "...", ".. ", "a/.. /b", "a/ /b", "a/.../b",
		"CON.", "NUL ", "con . ", // Windows strips trailing dots and spaces
	}
	for _, p := range bad {
		if err := SafeRemoteSharePath(p); err == nil {
			t.Errorf("SafeRemoteSharePath(%q) accepted", p)
		}
	}
	ok := []string{
		"", ".", "a/b", `a\b`, "/share/dir/file.txt", "a..b", "config.txt", "console", "console.log", "COM10", "LPT10", "COMX", "CONIN", "CLOCK", "a/./b", "a. b",
		"com", "lpt", "icon", "auxiliary", "nulled.txt", "my con", "a b/c d.txt", "dir/conf",
	}
	for _, p := range ok {
		if err := SafeRemoteSharePath(p); err != nil {
			t.Errorf("SafeRemoteSharePath(%q) rejected: %v", p, err)
		}
	}
}
