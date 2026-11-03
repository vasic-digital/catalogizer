package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"catalogizer/utils"

	obslogging "digital.vasic.observability/pkg/logging"
	"digital.vasic.security/pkg/guardrails"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// InputValidationConfig holds validation configuration
type InputValidationConfig struct {
	// Maximum request body size in bytes
	MaxRequestBodySize int64
	// Enable SQL injection detection
	EnableSQLInjectionDetection bool
	// Enable XSS detection
	EnableXSSDetection bool
	// Enable path traversal detection
	EnablePathTraversalDetection bool
	// Custom validation rules
	CustomRules map[string]string
	// GuardrailEngine is an optional digital.vasic.security guardrail engine.
	// When set, content is additionally validated against the module's rule set
	// (max length, forbidden patterns, etc.) before the built-in regex checks.
	GuardrailEngine *guardrails.Engine
}

// DefaultInputValidationConfig returns secure defaults
func DefaultInputValidationConfig() InputValidationConfig {
	return InputValidationConfig{
		MaxRequestBodySize:           10 * 1024 * 1024, // 10MB
		EnableSQLInjectionDetection:  true,
		EnableXSSDetection:           true,
		EnablePathTraversalDetection: true,
		CustomRules:                  make(map[string]string),
	}
}

// Pre-compiled injection detection patterns
var (
	xssRegexes           []*regexp.Regexp
	pathTraversalRegexes []*regexp.Regexp
)

func init() {
	// XSS patterns
	for _, p := range []string{
		`(?is)(<script[^>]*>.*?</script>)`,
		`(?is)(<iframe[^>]*>.*?</iframe>)`,
		`(?is)(<object[^>]*>.*?</object>)`,
		`(?is)(<embed[^>]*>.*?</embed>)`,
		// Open tags that load or execute content never need a closing tag or a
		// single-line body: an unclosed <script src=...>, a multi-line <script>
		// body, <base href>, <link>, <style>@import. Formerly caught only by
		// accident, through the bare word SCRIPT in the SQL keyword list.
		`(?i)<\s*/?\s*(script|iframe|object|embed|applet|base|meta|link|style|svg|math|frameset|form)\b`,
		`(?i)(javascript\s*:|vbscript\s*:|data\s*:|onload\s*=|onerror\s*=)`,
		`(?i)(on\w+\s*=)`,
		`(?i)(eval\s*\(|alert\s*\(|confirm\s*\(|prompt\s*\()`,
	} {
		xssRegexes = append(xssRegexes, regexp.MustCompile(p))
	}

	// Path traversal patterns
	for _, p := range []string{
		`\.\./|\.\.\\`,
		`%2e%2e%2f|%2e%2e\\`,
		`\.\.%2f|\.\.%5c`,
		`%2e%2e%5c|%2e%2e/`,
		`/etc/passwd|/etc/shadow|/etc/hosts`,
		`windows/system32|boot\.ini|win\.ini`,
	} {
		pathTraversalRegexes = append(pathTraversalRegexes, regexp.MustCompile(p))
	}
}

// SanitizeInput performs basic sanitisation on input strings.
//
// FIX-QA-2026-04-21-009: the idempotency fuzzer
// (FuzzSanitizeInput_Idempotent) surfaced a case where
// SanitizeInput("\xdd 0") → " 0" (a leading space appears once the
// invalid UTF-8 byte is dropped), but a second pass produced "0"
// (TrimSpace now sees the leading space). Non-idempotent sanitisers
// are dangerous: any multi-hop pipeline that re-sanitises (e.g.,
// server-side after a CSRF middleware already did so) mutates data
// between hops. Fix: move TrimSpace to run LAST so the result is a
// fixed point under repeated application.
func SanitizeInput(input string) string {
	// Remove null bytes first — they're never meaningful payload.
	input = strings.ReplaceAll(input, "\x00", "")

	// Ensure valid UTF-8. Drop invalid leading/continuation bytes,
	// which can leave whitespace next to formerly-adjacent bytes.
	if !utf8.ValidString(input) {
		valid := make([]rune, 0, len(input))
		for i, r := range input {
			if r == utf8.RuneError {
				_, size := utf8.DecodeRuneInString(input[i:])
				if size == 1 {
					continue
				}
			}
			valid = append(valid, r)
		}
		input = string(valid)
	}

	// TrimSpace LAST so stripping invalid bytes cannot leave
	// surrounding whitespace that changes the value on a second
	// pass.
	return strings.TrimSpace(input)
}

// DetectSQLInjection checks for common SQL injection patterns
// Detection is structural (see utils.ContainsSQLInjection): bare SQL keywords
// such as the "create"/"update"/"delete" in the permission names
// "user.create"/"user.update"/"user.delete" are not injection.
func DetectSQLInjection(input string) bool {
	return utils.ContainsSQLInjection(input)
}

// DetectXSS checks for common XSS patterns
func DetectXSS(input string) bool {
	for _, re := range xssRegexes {
		if re.MatchString(input) {
			return true
		}
	}
	return false
}

// DetectPathTraversal checks for path traversal attempts
func DetectPathTraversal(input string) bool {
	for _, re := range pathTraversalRegexes {
		if re.MatchString(input) {
			return true
		}
	}
	return false
}

// ValidateRequestBody validates request body against security rules
func ValidateRequestBody(config InputValidationConfig, c *gin.Context) error {
	// Check content length
	if c.Request.ContentLength > config.MaxRequestBodySize {
		return fmt.Errorf("request body too large")
	}

	// Only validate specific content types and methods that typically have bodies
	contentType := c.GetHeader("Content-Type")
	method := c.Request.Method

	// Skip validation for GET requests (they shouldn't have bodies)
	if method == "GET" || method == "DELETE" || method == "HEAD" {
		return nil
	}

	// Skip validation for non-form/json content types
	if contentType != "" &&
		!strings.Contains(contentType, "application/json") &&
		!strings.Contains(contentType, "application/x-www-form-urlencoded") &&
		!strings.Contains(contentType, "multipart/form-data") {
		return nil // Skip validation for other content types
	}

	// Read and parse body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		return fmt.Errorf("failed to read request body")
	}

	if len(bodyBytes) > int(config.MaxRequestBodySize) {
		return fmt.Errorf("request body too large")
	}

	// Restore the request body for subsequent handlers
	c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	// For JSON requests, validate the structure
	if strings.Contains(contentType, "application/json") {
		var jsonData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
			return fmt.Errorf("invalid JSON format")
		}

		// Recursively validate all string values
		return validateJSONValues(config, jsonData)
	}

	return nil
}

// validateJSONValues recursively validates all string values in JSON
func validateJSONValues(config InputValidationConfig, data map[string]interface{}) error {
	return validateJSONValuesAt(config, data, true)
}

// validateJSONValuesAt walks the JSON object. topLevel is true only for the
// request body's own keys: credential fields (secretFieldNames) are exempt from
// pattern checks there and nowhere deeper, so a nested "password" key (for
// example inside a storage-root config echoed back to the admin UI) is checked.
func validateJSONValuesAt(config InputValidationConfig, data map[string]interface{}, topLevel bool) error {
	for key, value := range data {
		switch v := value.(type) {
		case string:
			if topLevel && isSecretField(key) {
				if err := validateSecretValue(config, key, v); err != nil {
					return err
				}
				continue
			}
			if err := validateStringValue(config, key, v); err != nil {
				return err
			}
		case map[string]interface{}:
			if err := validateJSONValuesAt(config, v, false); err != nil {
				return err
			}
		case []interface{}:
			for i, item := range v {
				if str, ok := item.(string); ok {
					ik := fmt.Sprintf("%s[%d]", key, i)
					if topLevel && isSecretField(key) {
						if err := validateSecretValue(config, ik, str); err != nil {
							return err
						}
						continue
					}
					if err := validateStringValue(config, ik, str); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// secretFieldNames are JSON keys whose values are credentials. They are hashed
// (bcrypt) and never interpolated into SQL, HTML or a file path, and a strong
// password legitimately contains any character or word ("Create", "Select-1",
// "a--b", "x=1"). Pattern-checking them can only reject valid passwords, so they
// are exempt from the injection pattern checks (length is bounded by the body
// limit and by AuthService.ValidatePassword). All other fields keep the checks.
var secretFieldNames = map[string]struct{}{
	"password":         {},
	"new_password":     {},
	"current_password": {},
	"old_password":     {},
	"confirm_password": {},
}

// isSecretField reports whether the (possibly "name[3]"-indexed) key is a credential field.
func isSecretField(key string) bool {
	if i := strings.IndexByte(key, '['); i >= 0 {
		key = key[:i]
	}
	_, ok := secretFieldNames[strings.ToLower(key)]
	return ok
}

// validateSecretValue applies only the operator-configured rule (if any) to a
// credential value; no pattern check (see secretFieldNames).
func validateSecretValue(config InputValidationConfig, key, value string) error {
	pattern, exists := config.CustomRules[key]
	if !exists {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid validation pattern for field %s", key)
	}
	if !re.MatchString(value) {
		return fmt.Errorf("field %s does not match required pattern", key)
	}
	return nil
}

// validateStringValue validates individual string values
func validateStringValue(config InputValidationConfig, key, value string) error {
	// Sanitize input
	value = SanitizeInput(value)

	// Run digital.vasic.security guardrail checks when engine is configured.
	// This adds module-based content validation (max length, forbidden patterns)
	// before the built-in regex checks below.
	if config.GuardrailEngine != nil {
		if result := config.GuardrailEngine.Check(value); !result.Passed {
			return fmt.Errorf("guardrail violation in field %s: %s", key, result.Results[0].Error)
		}
	}

	// Check for SQL injection
	if config.EnableSQLInjectionDetection && DetectSQLInjection(value) {
		return fmt.Errorf("potential SQL injection detected in field: %s", key)
	}

	// Check for XSS
	if config.EnableXSSDetection && DetectXSS(value) {
		return fmt.Errorf("potential XSS detected in field: %s", key)
	}

	// Check for path traversal
	if config.EnablePathTraversalDetection && DetectPathTraversal(value) {
		return fmt.Errorf("potential path traversal detected in field: %s", key)
	}

	// Check custom rules
	if pattern, exists := config.CustomRules[key]; exists {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid validation pattern for field %s", key)
		}
		if !re.MatchString(value) {
			return fmt.Errorf("field %s does not match required pattern", key)
		}
	}

	return nil
}

// InputValidation creates a middleware for input validation
func InputValidation(config InputValidationConfig) gin.HandlerFunc {
	if config.MaxRequestBodySize == 0 {
		config = DefaultInputValidationConfig()
	}

	return func(c *gin.Context) {
		// Validate request body
		if err := ValidateRequestBody(config, c); err != nil {
			// Log the attempt using digital.vasic.observability structured logging.
			// This replaces the former fmt.Printf with JSON-formatted, field-rich output.
			obslogging.NewLogrusAdapter(obslogging.DefaultConfig()).
				WithField("ip", c.ClientIP()).
				WithField("path", c.Request.URL.Path).
				WithError(err).
				Warn("Security validation failed")

			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "validation_failed",
				"message": "Invalid input detected",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Add security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		c.Next()
	}
}

// ValidateStruct validates a struct using the validator library
func ValidateStruct(obj interface{}) error {
	return validate.Struct(obj)
}

// ValidateVar validates a single field
func ValidateVar(field interface{}, tag string) error {
	return validate.Var(field, tag)
}
