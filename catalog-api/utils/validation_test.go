package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringValidator_Validate(t *testing.T) {
	validator := NewStringValidator(3, 10, "^[a-z]+$")

	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid string", "hello", true},
		{"too short", "ab", false},
		{"too long", "helloworld123", false},
		{"invalid chars", "hello123", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.input)
			assert.Equal(t, tt.wantValid, result.Valid)
		})
	}
}

func TestIsAlphanumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123", true},
		{"ABC", true},
		{"123", true},
		{"abc-123", false},
		{"abc 123", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAlphanumeric(tt.input))
		})
	}
}

func TestIsSafeString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Hello World!", true},
		{"Test-123_test.", true},
		{"Hello<script>", false},
		{"Hello|World", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSafeString(tt.input))
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"test@example.com", true},
		{"user.name@example.co.uk", true},
		{"user+tag@example.com", true},
		{"invalid", false},
		{"@example.com", false},
		{"test@", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidEmail(tt.input))
		})
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"550E8400-E29B-41D4-A716-446655440000", true},
		{"550e8400e29b41d4a716446655440000", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidUUID(tt.input))
		})
	}
}

func TestIsSafeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"file.txt", true},
		{"document.pdf", true},
		{"../etc/passwd", false},
		{"file/../../passwd", false},
		{"file:test", false},
		{"", false}, // Empty filename is not valid (regex requires 1+ char)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsSafeFilename(tt.input))
		})
	}
}

func TestIsValidHexColor(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"#FFFFFF", true},
		{"#ffffff", true},
		{"#FFF", true},
		{"#123456", true},
		{"FFFFFF", false},
		{"#GGGGGG", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidHexColor(tt.input))
		})
	}
}

func TestIsValidIPv4(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"256.1.1.1", false},
		{"192.168.1", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidIPv4(tt.input))
		})
	}
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"http://example.com", true},
		{"https://example.com/path", true},
		{"https://example.com:8080/path?query=1", true},
		{"ftp://example.com", false},
		{"not-a-url", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidURL(tt.input))
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"hello\x00world", "helloworld"},
		{"hello\nworld", "hello\nworld"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeString(tt.input))
		})
	}
}

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<script>alert('xss')</script>", "alert('xss')"},
		{"Hello <b>World</b>", "Hello World"},
		{"Plain text", "Plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeHTML(tt.input))
		})
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<script>", "&lt;script&gt;"},
		{"hello & world", "hello &amp; world"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"'single'", "&#x27;single&#x27;"},
		{"normal", "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, EscapeHTML(tt.input))
		})
	}
}

func TestContainsSQLInjection(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"SELECT * FROM users", true},
		{"'; DROP TABLE users; --", true},
		{"admin' OR '1'='1", true}, // quote + OR tautology
		{"normal text", false},
		{"selection", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ContainsSQLInjection(tt.input))
		})
	}
}

func TestContainsXSS(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"<script>alert('xss')</script>", true},
		{"javascript:alert('xss')", true},
		{"<img onerror=alert('xss')>", true},
		{"normal text", false},
		{"<b>bold</b>", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ContainsXSS(tt.input))
		})
	}
}

func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"../../../etc/passwd", true},
		{"..\\windows\\system32", true},
		{"file.txt", false},
		{"/path/to/file", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ContainsPathTraversal(tt.input))
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 2, "hi"},
		{"hello", 3, "hel"}, // maxLen <= 3: truncates without ellipsis
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, Truncate(tt.input, tt.maxLen))
		})
	}
}

func TestRemoveWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "helloworld"},
		{"  hello   world  ", "helloworld"},
		{"hello\nworld\t!", "helloworld!"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, RemoveWhitespace(tt.input))
		})
	}
}

func TestCompactWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello   world", "hello world"},
		{"  hello   world  ", "hello world"},
		{"hello\n\n\nworld", "hello world"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, CompactWhitespace(tt.input))
		})
	}
}

// Regression (root cause: bare-keyword matching on word boundaries): permission
// names and ordinary prose must not be flagged, real injection must be.
func TestContainsSQLInjection_PermissionNamesAndProse(t *testing.T) {
	benign := []string{
		"user.create", "user.update", "user.delete", "role.create", "media.delete",
		"select", "from", "or", "and", "create", "Users and groups", "Cats or dogs",
		"Can create and update items", "Order by popularity", "where the heart is",
		"system.admin", "media.view",
	}
	for _, in := range benign {
		assert.False(t, ContainsSQLInjection(in), in)
	}
	attacks := []string{
		"'; DROP TABLE users; --", "' OR '1'='1", "x' OR 1=1", "1 OR 2>1",
		"1 UNION SELECT a FROM b", "1 UNION ALL SELECT 1", "SELECT * FROM users",
		"INSERT INTO t VALUES (1)", "UPDATE users SET admin=1", "DELETE FROM users",
		"admin'--", "1; DROP TABLE x", "1; SLEEP(5)", "WAITFOR DELAY '0:0:5'",
		"BENCHMARK(5000000,SHA1('t'))", "EXEC(xp_cmdshell 'dir')", "EXEC xp_cmdshell",
		"1/**/OR/**/1=1", "1; SELECT 1", "pg_sleep(10)", "ALTER TABLE t ADD c int",
	}
	for _, in := range attacks {
		assert.True(t, ContainsSQLInjection(in), in)
	}
}

func TestContainsSQLInjection_Round2Additions(t *testing.T) {
	for _, in := range []string{
		`1 AND (SELECT COUNT(*) FROM users) > 0`,
		`1 OR 1=1`, `x' OR 'a'='a`, `1 GROUP BY a HAVING 1=1`,
		`1 AND extractvalue(1,concat(0x7e,version()))`,
		`1; DROP TABLE users`, `' UNION SELECT password FROM users --`,
	} {
		assert.True(t, ContainsSQLInjection(in), "must detect: %q", in)
	}
	for _, in := range []string{"user.create", "user.update", "having a good time", "select your plan", "Dr0p-table#Throwaway"} {
		assert.False(t, ContainsSQLInjection(in), "must accept: %q", in)
	}
}
