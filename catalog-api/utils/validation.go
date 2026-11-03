package utils

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	Valid   bool
	Errors  []string
	Cleaned string
}

// StringValidator provides common string validation functions
type StringValidator struct {
	MaxLength int
	MinLength int
	Pattern   *regexp.Regexp
}

// NewStringValidator creates a new string validator with constraints
func NewStringValidator(minLen, maxLen int, pattern string) *StringValidator {
	sv := &StringValidator{
		MinLength: minLen,
		MaxLength: maxLen,
	}

	if pattern != "" {
		sv.Pattern = regexp.MustCompile(pattern)
	}

	return sv
}

// Validate checks if a string meets the validation criteria
func (sv *StringValidator) Validate(input string) ValidationResult {
	result := ValidationResult{
		Valid:   true,
		Errors:  []string{},
		Cleaned: strings.TrimSpace(input),
	}

	// Check minimum length
	if sv.MinLength > 0 && len(result.Cleaned) < sv.MinLength {
		result.Valid = false
		result.Errors = append(result.Errors,
			"Input is too short")
	}

	// Check maximum length
	if sv.MaxLength > 0 && len(result.Cleaned) > sv.MaxLength {
		result.Valid = false
		result.Errors = append(result.Errors,
			"Input is too long")
	}

	// Check pattern
	if sv.Pattern != nil && !sv.Pattern.MatchString(result.Cleaned) {
		result.Valid = false
		result.Errors = append(result.Errors,
			"Input contains invalid characters")
	}

	return result
}

// Common validation patterns
var (
	// Alphanumeric only
	AlphaNumericPattern = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	// Alphanumeric with spaces and basic punctuation
	SafeStringPattern = regexp.MustCompile(`^[a-zA-Z0-9\s\-_.,!?():;@]+$`)

	// Email validation (basic)
	EmailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// UUID validation
	UUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	// Safe filename (no path traversal)
	SafeFilenamePattern = regexp.MustCompile(`^[^/\\:*?"<>|]+$`)

	// Hex color
	HexColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

	// IP address (IPv4)
	IPv4Pattern = regexp.MustCompile(`^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)

	// URL (basic)
	URLPattern = regexp.MustCompile(`^https?://[\w\-\.]+(:\d+)?(/[\w\-\./?%&=]*)?$`)
)

// IsAlphanumeric checks if string contains only letters and numbers
func IsAlphanumeric(input string) bool {
	return AlphaNumericPattern.MatchString(input)
}

// IsSafeString checks if string contains only safe characters
func IsSafeString(input string) bool {
	return SafeStringPattern.MatchString(input)
}

// IsValidEmail validates email format
func IsValidEmail(input string) bool {
	return EmailPattern.MatchString(input)
}

// IsValidUUID validates UUID format
func IsValidUUID(input string) bool {
	return UUIDPattern.MatchString(input)
}

// IsSafeFilename validates filename (no path traversal)
func IsSafeFilename(input string) bool {
	// Check pattern
	if !SafeFilenamePattern.MatchString(input) {
		return false
	}

	// Check for path traversal attempts
	if strings.Contains(input, "..") || strings.Contains(input, "//") {
		return false
	}

	return true
}

// IsValidHexColor validates hex color format
func IsValidHexColor(input string) bool {
	return HexColorPattern.MatchString(input)
}

// IsValidIPv4 validates IPv4 address format
func IsValidIPv4(input string) bool {
	return IPv4Pattern.MatchString(input)
}

// IsValidURL validates URL format
func IsValidURL(input string) bool {
	return URLPattern.MatchString(input)
}

// SanitizeString performs basic sanitisation.
//
// FIX-QA-2026-04-21-010: same anti-pattern as middleware.SanitizeInput
// (FIX-QA-2026-04-21-009) — `\xff\xfe invalid utf8` produced
// " invalid utf8" on pass 1 (invalid bytes dropped, leading space
// remains) and "invalid utf8" on pass 2 (TrimSpace catches it).
// TrimSpace MUST run after the UTF-8 cleanup so the result is a
// fixed point under repeated application.
func SanitizeString(input string) string {
	// Remove null bytes first.
	input = strings.ReplaceAll(input, "\x00", "")

	// Normalise Unicode — drop invalid sequences.
	if !utf8.ValidString(input) {
		input = strings.ToValidUTF8(input, "")
	}

	// TrimSpace LAST so stripping invalid bytes can't leave
	// adjacent whitespace that mutates the result between passes.
	return strings.TrimSpace(input)
}

// SanitizeHTML removes HTML tags
func SanitizeHTML(input string) string {
	// Simple tag removal
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(input, "")
}

// EscapeHTML escapes HTML special characters
func EscapeHTML(input string) string {
	input = strings.ReplaceAll(input, "&", "&amp;")
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, `"`, "&quot;")
	input = strings.ReplaceAll(input, "'", "&#x27;")
	return input
}

// sqlInjectionPatterns detect SQL injection by STRUCTURE (a statement shape,
// a tautology, a stacked statement, a comment terminator, a time-delay or
// stored-procedure call), never by a bare keyword.
//
// Root cause of the former false positive: the old list matched a lone
// `\b(SELECT|INSERT|UPDATE|DELETE|CREATE|DROP|OR|AND|FROM|WHERE|...)\b`.
// `\b` is a word/non-word boundary, and '.' is a non-word character, so the
// legitimate permission names "user.create", "user.update", "user.delete" (and
// any prose such as "Users and groups") contained a "word" that equalled a SQL
// keyword and were rejected as injection. A bare keyword carries no injection
// capability on its own: injection needs a statement, an operator expression or
// a terminator around it, so those are what is matched now.
var sqlInjectionPatterns = []*regexp.Regexp{
	// UNION [ALL] SELECT followed by something that looks like a select list
	// (so the prose "the union select committee" is not injection).
	regexp.MustCompile(`(?i)\bUNION\b(\s+(ALL|DISTINCT))?\s+SELECT\b(\s*(\*|\d|NULL\b|@@|['"(\x60])|\s+[\w.\x60]+\s*(,|\(|\bFROM\b|--|#|/\*|;)|\s*(--|#|/\*|;))`),
	// SELECT <list> FROM <source>
	regexp.MustCompile(`(?i)\bSELECT\s+(\*|[\w.` + "`" + `"'(),\s]+?)\s+FROM\s+[\w.` + "`" + `"\[]+`),
	// INSERT INTO, DELETE FROM, UPDATE <t> SET
	regexp.MustCompile(`(?i)\bINSERT\s+(OR\s+\w+\s+)?INTO\b`),
	regexp.MustCompile(`(?i)\bDELETE\s+FROM\b`),
	regexp.MustCompile(`(?i)\bUPDATE\s+[\w.` + "`" + `"\[\]]+\s+SET\b`),
	// DDL: DROP/ALTER/CREATE/TRUNCATE <object>
	regexp.MustCompile(`(?i)\b(DROP|ALTER|CREATE|TRUNCATE)\s+(TABLE|DATABASE|SCHEMA|INDEX|VIEW|USER|ROLE|TRIGGER|PROCEDURE|FUNCTION|COLUMN)\b`),
	// Tautologies: quote OR/AND ..., or OR/AND <literal> <comparison>
	regexp.MustCompile(`(?i)['"]\s*(OR|AND)\s+['"\w(]`),
	regexp.MustCompile(`(?i)\b(OR|AND)\s+(\d+|'[^']*'|"[^"]*")\s*(=|<>|!=|<|>|\bLIKE\b)`),
	// Sub-selects, SELECT*FROM, boolean tautologies, parenthesised OR/AND, HAVING, DB error/IO functions
	regexp.MustCompile(`(?i)\(\s*SELECT\b`),
	regexp.MustCompile(`(?i)\bSELECT\s*\*\s*FROM\b`),
	regexp.MustCompile(`(?i)['")\d]\s*(OR|AND)\s*\(`),
	regexp.MustCompile(`(?i)['")\d]\s*(OR|AND)\s+(TRUE|FALSE)\b`),
	regexp.MustCompile(`(?i)\bHAVING\s+\d+\s*=\s*\d+`),
	regexp.MustCompile(`(?i)\b(EXTRACTVALUE|UPDATEXML|LOAD_FILE|PG_READ_FILE|SYS_EXEC|SYS_EVAL)\s*\(`),
	// Stacked statement: ; <statement keyword>
	regexp.MustCompile(`(?i);\s*(SELECT|INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|EXEC|EXECUTE)\b`),
	// Stacked time-delay / shutdown: only in call/terminated form so prose such
	// as "Loop; sleep well" or "Chapter 3; shutdown sequence" is not blocked.
	regexp.MustCompile(`(?i);\s*(SLEEP|PG_SLEEP|BENCHMARK)\s*\(`),
	regexp.MustCompile(`(?i);\s*WAITFOR\s+(DELAY|TIME)\b`),
	regexp.MustCompile(`(?i);\s*SHUTDOWN\s*(;|--|#|/\*|$|\bWITH\s+NOWAIT\b)`),
	regexp.MustCompile(`(?i)(['"]\s*;\s*(SELECT|INSERT|UPDATE|DELETE|DROP))`),
	// Comment terminators / inline comments
	regexp.MustCompile(`(?i)(/\*.*\*/)`),
	// "--" / "#" comment terminators are judged by hasSQLCommentTerminator
	// (context-aware), not by a regexp. Unterminated block comment right after a
	// quote/paren ("admin'/*") stays a regexp.
	regexp.MustCompile(`['")]\s*/\*`),
	// ORDER BY <column number>: column-count probing ("1 order by 5").
	regexp.MustCompile(`(?i)['")\d]\s+ORDER\s+BY\s+\d+\s*(--|#|;|/\*|$)`),
	// Boolean/blind-extraction helpers and file/blob primitives
	regexp.MustCompile(`(?i)\b(SUBSTR|SUBSTRING|MID|ASCII|ORD|LENGTH)\s*\(\s*\(?\s*(SELECT|VERSION|USER|DATABASE|CURRENT_USER|@@)`),
	regexp.MustCompile(`(?i)\b(RANDOMBLOB|LOAD_EXTENSION|ZEROBLOB)\s*\(`),
	// String concatenation tautology: ' || '1'='1
	regexp.MustCompile(`['"]\s*(\|\||&&)\s*['"\w(]`),
	// MySQL PROCEDURE ANALYSE() information disclosure
	regexp.MustCompile(`(?i)\bPROCEDURE\s+ANALYSE\s*\(`),
	// Time delay / stored procedures / dangerous functions (call syntax)
	regexp.MustCompile(`(?i)\bWAITFOR\s+DELAY\b`),
	regexp.MustCompile(`(?i)\b(PG_)?SLEEP\s*\(\s*(\d+(\.\d+)?\s*[)*+\-]|\(?\s*SELECT\b|@)`),
	regexp.MustCompile(`(?i)\bBENCHMARK\s*\(`),
	regexp.MustCompile(`(?i)\bEXEC(UTE)?\s*\(`),
	regexp.MustCompile(`(?i)\bEXEC(UTE)?\s+(sp_|xp_|master\b)`),
	regexp.MustCompile(`(?i)\bXP_\w+\b`),
}

// ContainsSQLInjection checks for SQL injection by statement structure. It is
// the single implementation shared with the HTTP input-validation middleware.
func ContainsSQLInjection(input string) bool {
	if hasSQLCommentTerminator(input) {
		return true
	}
	for _, re := range sqlInjectionPatterns {
		if re.MatchString(input) {
			return true
		}
	}
	return false
}

// ContainsXSS checks for common XSS patterns
func ContainsXSS(input string) bool {
	xssPatterns := []string{
		`(?i)(<script[^>]*>.*?</script>)`,
		`(?i)(javascript\s*:)`,
		`(?i)(vbscript\s*:)`,
		`(?i)(onload\s*=|onerror\s*=|onmouseover\s*=)`,
		`(?i)(eval\s*\()`,
	}

	for _, pattern := range xssPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(input) {
			return true
		}
	}

	return false
}

// ContainsPathTraversal checks for path traversal attempts
func ContainsPathTraversal(input string) bool {
	// Check for common traversal patterns
	if strings.Contains(input, "../") || strings.Contains(input, "..\\") {
		return true
	}

	// Check for encoded traversal
	if strings.Contains(input, "%2e%2e") {
		return true
	}

	// Check for null byte injection
	if strings.Contains(input, "\x00") {
		return true
	}

	return false
}

// NormalizeUnicode normalizes Unicode input
func NormalizeUnicode(input string) string {
	// Remove control characters except common whitespace
	var result strings.Builder
	for _, r := range input {
		if unicode.IsControl(r) {
			if r == '\n' || r == '\r' || r == '\t' {
				result.WriteRune(r)
			}
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}

// Truncate truncates string to max length with ellipsis
func Truncate(input string, maxLen int) string {
	if len(input) <= maxLen {
		return input
	}

	if maxLen <= 3 {
		return input[:maxLen]
	}

	return input[:maxLen-3] + "..."
}

// RemoveWhitespace removes all whitespace
func RemoveWhitespace(input string) string {
	var result strings.Builder
	for _, r := range input {
		if !unicode.IsSpace(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// CompactWhitespace replaces multiple whitespace with single space
func CompactWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

// numericLiteralPrefix matches a prefix that is only a numeric literal: optional
// sign, optional opening parentheses, a decimal / 0x-hex number, optional closing
// parentheses ("1", "-1", "((5", "0x1", "1.5", "1))").
var numericLiteralPrefix = regexp.MustCompile(`^\s*[-+]?\s*\(*\s*[-+]?\s*(0[xX][0-9a-fA-F]+|\d+(\.\d+)?|\.\d+)\s*\)*$`)

// hasSQLCommentTerminator reports a "--" or "#" comment used as a terminator in
// ONE linear pass (running quote parity, no per-marker rescans: a per-marker
// strings.Count was O(n^2), 4 MiB took 46 s). Prose uses both markers ("Episode 3
// -- The Return", "Ep. 7 #", "Tag #1"), so the character before the marker decides:
//   - start of input, or ';'  -> terminator ("-- x", "1; -- x")
//   - quote / back-tick       -> terminator when that quote is unbalanced before it
//     ("admin' -- x"), for "#" only when the marker ends the input ("admin' #")
//   - ')' or a digit ("--")   -> terminator when nothing follows the marker, OR any
//     quote is unbalanced before it ("admin') -- x"), OR the text before it is only
//     a numeric literal ("1 -- x", "-1 -- x", "5--x", "0x1 -- x", "1.5 -- x",
//     "1)) -- x"). "Season 1 -- Episode 4" and "(live) -- remaster" stay legal.
//   - for "#": ')' counts only when the marker ends the input; a digit never counts.
//
// Known, accepted false positive (security over prose, operator decision
// candidate): a purely numeric title followed by "--", e.g. "2001 -- A Space Odyssey".
func hasSQLCommentTerminator(s string) bool {
	n := len(s)
	// lastSigDash: last index of a char that is not whitespace and not '-'
	// (what "--" must be followed by to count as "something follows");
	// lastSigHash: last index of a non-whitespace char.
	lastSigDash, lastSigHash := -1, -1
	for i := n - 1; i >= 0 && (lastSigDash < 0 || lastSigHash < 0); i-- {
		c := s[i]
		ws := c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
		if !ws && lastSigHash < 0 {
			lastSigHash = i
		}
		if !ws && c != '-' && lastSigDash < 0 {
			lastSigDash = i
		}
	}
	var sq, dq, bq int           // quote counts so far
	prev, prevIdx := byte(0), -1 // last non-space byte before the scan position
	numericPossible := true
	for i := 0; i < n; i++ {
		c := s[i]
		isDash := c == '-' && i+1 < n && s[i+1] == '-'
		if isDash || c == '#' {
			from := i + 1
			if isDash {
				from = i + 2
			}
			restEmpty := lastSigHash < from
			if isDash {
				restEmpty = lastSigDash < from
			}
			switch {
			case prevIdx < 0:
				if isDash {
					return true
				}
			case prev == ';':
				return true
			case prev == '\'' || prev == '"' || prev == '`':
				cnt := map[byte]int{'\'': sq, '"': dq, '`': bq}[prev]
				if cnt%2 == 1 && (isDash || restEmpty) {
					return true
				}
			case prev == ')' || (prev >= '0' && prev <= '9' && isDash):
				if restEmpty {
					return true
				}
				if isDash {
					if sq%2 == 1 || dq%2 == 1 || bq%2 == 1 {
						return true
					}
					if numericPossible {
						if numericLiteralPrefix.MatchString(s[:prevIdx+1]) {
							return true
						}
						numericPossible = false // any longer prefix still holds the bad part
					}
				}
			}
			if isDash {
				i++ // skip the second '-'
				prev, prevIdx = '-', i
				continue
			}
		}
		switch c {
		case '\'':
			sq++
		case '"':
			dq++
		case '`':
			bq++
		}
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '\v' && c != '\f' {
			prev, prevIdx = c, i
		}
	}
	return false
}
