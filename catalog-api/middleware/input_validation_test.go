package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInputValidation_SQLInjectionDetection(t *testing.T) {
	// Test SQL injection patterns
	testCases := []struct {
		name  string
		input string
		valid bool
	}{
		{"Valid text", "Hello world", true},
		{"SQL SELECT", "SELECT * FROM users", false},
		{"SQL UNION", "1' UNION SELECT password FROM users", false},
		{"SQL COMMENT", "admin'--", false},
		{"SQL OR", "' OR '1'='1", false},
		{"SQL DROP", "DROP TABLE users", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectSQLInjection(tc.input)
			assert.Equal(t, tc.valid, !result)
		})
	}
}

func TestInputValidation_XSSDetection(t *testing.T) {
	// Test XSS patterns
	testCases := []struct {
		name  string
		input string
		valid bool
	}{
		{"Valid text", "Hello world", true},
		{"Script tag", "<script>alert('xss')</script>", false},
		{"Iframe", "<iframe src='evil.com'></iframe>", false},
		{"JavaScript", "javascript:alert('xss')", false},
		{"Onload", "<img onload='alert(1)'>", false},
		{"Eval", "eval('malicious code')", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectXSS(tc.input)
			assert.Equal(t, tc.valid, !result)
		})
	}
}

func TestInputValidation_PathTraversalDetection(t *testing.T) {
	// Test path traversal patterns
	testCases := []struct {
		name  string
		input string
		valid bool
	}{
		{"Valid path", "/home/user/file.txt", true},
		{"Basic traversal", "../../../etc/passwd", false},
		{"URL encoded traversal", "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd", false},
		{"Windows traversal", "..\\..\\..\\windows\\system32\\config\\sam", false},
		{"Unix traversal", "..././.../.../etc/passwd", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectPathTraversal(tc.input)
			assert.Equal(t, tc.valid, !result)
		})
	}
}

func TestInputValidation_GinMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create router with validation middleware
	router := gin.New()
	router.Use(InputValidation(DefaultInputValidationConfig()))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test SQL injection
	t.Run("Reject SQL Injection", func(t *testing.T) {
		jsonData := `{"search": "SELECT * FROM users"}`
		req := httptest.NewRequest("POST", "/test", strings.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "potential SQL injection")
	})

	// Test XSS
	t.Run("Reject XSS", func(t *testing.T) {
		jsonData := `{"comment": "<script>alert('xss')</script>"}`
		req := httptest.NewRequest("POST", "/test", strings.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, strings.ToLower(w.Body.String()), "potential xss")
	})

	// Test valid input
	t.Run("Allow valid input", func(t *testing.T) {
		jsonData := `{"name": "John Doe", "email": "john@example.com"}`
		req := httptest.NewRequest("POST", "/test", strings.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
	})
}

func TestInputValidation_SizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := DefaultInputValidationConfig()
	config.MaxRequestBodySize = 100 // Very small limit

	router := gin.New()
	router.Use(InputValidation(config))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test oversized body
	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = 'a'
	}

	jsonData := `{"data": "` + string(largeData) + `"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "request body too large")
}

func TestInputValidation_Sanitization(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"hello\x00world", "helloworld"},
		{"正常文字", "正常文字"}, // Valid Unicode
	}

	for _, tc := range testCases {
		t.Run("Sanitize input", func(t *testing.T) {
			result := SanitizeInput(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestInputValidation_SecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(InputValidation(DefaultInputValidationConfig()))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

// SanitizeInput additional coverage

func TestSanitizeInput_InvalidUTF8(t *testing.T) {
	// Create a string with invalid UTF-8 byte sequence
	invalidUTF8 := "hello\xc0\xafworld"
	result := SanitizeInput(invalidUTF8)
	// Should remove invalid sequences
	assert.NotContains(t, result, "\xc0")
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "world")
}

func TestSanitizeInput_NullBytesAndWhitespace(t *testing.T) {
	input := "  \x00hello\x00  "
	result := SanitizeInput(input)
	assert.Equal(t, "hello", result)
}

func TestSanitizeInput_EmptyString(t *testing.T) {
	result := SanitizeInput("")
	assert.Equal(t, "", result)
}

func TestSanitizeInput_OnlyWhitespace(t *testing.T) {
	result := SanitizeInput("   \t\n  ")
	assert.Equal(t, "", result)
}

func TestSanitizeInput_ValidUTF8PassesThrough(t *testing.T) {
	input := "Hello, 世界! 🌍"
	result := SanitizeInput(input)
	assert.Equal(t, input, result)
}

// validateJSONValues edge cases

func TestValidateJSONValues_NestedObjects(t *testing.T) {
	config := DefaultInputValidationConfig()
	data := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "SELECT * FROM users",
		},
	}
	err := validateJSONValues(config, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "potential SQL injection")
}

func TestValidateJSONValues_ArrayOfStrings(t *testing.T) {
	config := DefaultInputValidationConfig()
	data := map[string]interface{}{
		"tags": []interface{}{"<script>alert(1)</script>", "normal"},
	}
	err := validateJSONValues(config, data)
	assert.Error(t, err)
	// An XSS payload is reported as XSS (the old bare-keyword SQL rule mislabelled
	// it as SQL injection because of the word "script").
	assert.Contains(t, err.Error(), "potential XSS")
}

func TestValidateJSONValues_ArrayWithSafeStrings(t *testing.T) {
	config := DefaultInputValidationConfig()
	data := map[string]interface{}{
		"tags": []interface{}{"music", "rock", "classic"},
	}
	err := validateJSONValues(config, data)
	assert.NoError(t, err)
}

func TestValidateJSONValues_MixedTypes(t *testing.T) {
	config := DefaultInputValidationConfig()
	data := map[string]interface{}{
		"name":     "John Doe",
		"age":      float64(30),
		"is_admin": true,
		"scores":   []interface{}{float64(1), float64(2), float64(3)},
		"address": map[string]interface{}{
			"city":    "New York",
			"country": "US",
		},
	}
	err := validateJSONValues(config, data)
	assert.NoError(t, err)
}

func TestValidateJSONValues_PathTraversalInNested(t *testing.T) {
	config := DefaultInputValidationConfig()
	data := map[string]interface{}{
		"config": map[string]interface{}{
			"path": "../../../etc/passwd",
		},
	}
	err := validateJSONValues(config, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "potential path traversal")
}

func TestValidateJSONValues_XSSInArray(t *testing.T) {
	config := DefaultInputValidationConfig()
	data := map[string]interface{}{
		"comments": []interface{}{"great", "javascript:alert('xss')"},
	}
	err := validateJSONValues(config, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "potential XSS")
}

func TestValidateJSONValues_EmptyMap(t *testing.T) {
	config := DefaultInputValidationConfig()
	data := map[string]interface{}{}
	err := validateJSONValues(config, data)
	assert.NoError(t, err)
}

func TestValidateJSONValues_CustomRuleMatch(t *testing.T) {
	config := DefaultInputValidationConfig()
	config.CustomRules = map[string]string{
		"email": `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
	}
	data := map[string]interface{}{
		"email": "user@example.com",
	}
	err := validateJSONValues(config, data)
	assert.NoError(t, err)
}

func TestValidateJSONValues_CustomRuleNoMatch(t *testing.T) {
	config := DefaultInputValidationConfig()
	config.CustomRules = map[string]string{
		"zipcode": `^\d{5}$`,
	}
	data := map[string]interface{}{
		"zipcode": "not-a-zip",
	}
	err := validateJSONValues(config, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match required pattern")
}

// Additional middleware tests for various attack payloads

func TestInputValidation_PathTraversalViaMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(InputValidation(DefaultInputValidationConfig()))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	jsonData := `{"file_path": "../../../etc/passwd"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "path traversal")
}

func TestInputValidation_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(InputValidation(DefaultInputValidationConfig()))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid JSON format")
}

// D7/D8: an empty (or whitespace-only) JSON body has nothing to validate; whether a body is required
// is the handler's decision (after authentication), so the middleware must not answer 400 for it.
func TestInputValidation_EmptyJSONBodyIsNotMalformed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(InputValidation(DefaultInputValidationConfig()))
	router.POST("/test", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		c.JSON(http.StatusOK, gin.H{"body_len": len(b)})
	})
	for _, body := range []string{"", " ", "\n\t "} {
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "body %q: %s", body, w.Body.String())
	}
	// control: malformed non-empty JSON is still refused
	req := httptest.NewRequest("POST", "/test", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	// control: a whitespace-prefixed NON-empty body with an XSS payload is still refused
	req = httptest.NewRequest("POST", "/test", strings.NewReader(" {\"x\":\"<script>alert(1)</script>\"}"))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInputValidation_GETRequestSkipsBodyValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(InputValidation(DefaultInputValidationConfig()))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInputValidation_NonJSONContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(InputValidation(DefaultInputValidationConfig()))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("some binary data"))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInputValidation_ZeroConfigUsesDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Pass a zero-value config to trigger the default
	router := gin.New()
	router.Use(InputValidation(InputValidationConfig{}))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	jsonData := `{"name": "John"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInputValidation_ContentLengthExceedsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := DefaultInputValidationConfig()
	config.MaxRequestBodySize = 50

	router := gin.New()
	router.Use(InputValidation(config))

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	jsonData := `{"data": "this is a sufficiently long piece of text to exceed the limit of 50 bytes"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(jsonData))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "request body too large")
}

func TestInputValidation_SQLInjectionWithWaitfor(t *testing.T) {
	assert.True(t, DetectSQLInjection("WAITFOR DELAY '0:0:5'"))
}

func TestInputValidation_SQLInjectionWithBenchmark(t *testing.T) {
	assert.True(t, DetectSQLInjection("BENCHMARK(5000000,SHA1('test'))"))
}

func TestInputValidation_SQLInjectionWithSleep(t *testing.T) {
	assert.True(t, DetectSQLInjection("1; SLEEP(5)"))
}

func TestInputValidation_SQLInjectionWithExec(t *testing.T) {
	assert.True(t, DetectSQLInjection("EXEC(xp_cmdshell 'dir')"))
}

func TestInputValidation_XSSWithObjectTag(t *testing.T) {
	assert.True(t, DetectXSS("<object data='evil.swf'></object>"))
}

func TestInputValidation_XSSWithEmbedTag(t *testing.T) {
	assert.True(t, DetectXSS("<embed src='evil.swf'></embed>"))
}

func TestInputValidation_XSSWithEventHandler(t *testing.T) {
	assert.True(t, DetectXSS(`<div onmouseover="alert(1)">`))
}

func TestInputValidation_XSSWithConfirm(t *testing.T) {
	assert.True(t, DetectXSS("confirm('are you sure?')"))
}

func TestInputValidation_XSSWithPrompt(t *testing.T) {
	assert.True(t, DetectXSS("prompt('enter value')"))
}

func TestInputValidation_XSSWithDataURI(t *testing.T) {
	assert.True(t, DetectXSS("data:text/html,<script>alert(1)</script>"))
}

func TestInputValidation_XSSWithVBScript(t *testing.T) {
	assert.True(t, DetectXSS("vbscript:msgbox(1)"))
}

func TestInputValidation_PathTraversalURLEncoded(t *testing.T) {
	assert.True(t, DetectPathTraversal("%2e%2e%5c%2e%2e%5cwindows"))
}

func TestInputValidation_PathTraversalWindowsSystem32(t *testing.T) {
	assert.True(t, DetectPathTraversal("c:/windows/system32/config"))
}

func TestInputValidation_PathTraversalWinIni(t *testing.T) {
	assert.True(t, DetectPathTraversal("c:\\win.ini"))
}

func TestInputValidation_PathTraversalBootIni(t *testing.T) {
	assert.True(t, DetectPathTraversal("c:\\boot.ini"))
}

func TestInputValidation_PathTraversalEtcShadow(t *testing.T) {
	assert.True(t, DetectPathTraversal("/etc/shadow"))
}

func TestInputValidation_PathTraversalEtcHosts(t *testing.T) {
	assert.True(t, DetectPathTraversal("/etc/hosts"))
}

func TestInputValidation_SafeStringPassesAll(t *testing.T) {
	input := "This is a perfectly normal string with no attacks."
	assert.False(t, DetectSQLInjection(input))
	assert.False(t, DetectXSS(input))
	assert.False(t, DetectPathTraversal(input))
}

// ValidateStruct and ValidateVar coverage

func TestValidateStruct(t *testing.T) {
	type TestStruct struct {
		Name  string `validate:"required"`
		Email string `validate:"required,email"`
	}

	// Valid struct
	err := ValidateStruct(TestStruct{Name: "John", Email: "john@example.com"})
	assert.NoError(t, err)

	// Invalid struct
	err = ValidateStruct(TestStruct{Name: "", Email: "not-an-email"})
	assert.Error(t, err)
}

func TestValidateVar(t *testing.T) {
	// Valid email
	err := ValidateVar("john@example.com", "email")
	assert.NoError(t, err)

	// Invalid email
	err = ValidateVar("not-an-email", "email")
	assert.Error(t, err)
}

func TestDefaultInputValidationConfig(t *testing.T) {
	config := DefaultInputValidationConfig()
	assert.Equal(t, int64(10*1024*1024), config.MaxRequestBodySize)
	assert.True(t, config.EnableSQLInjectionDetection)
	assert.True(t, config.EnableXSSDetection)
	assert.True(t, config.EnablePathTraversalDetection)
	assert.NotNil(t, config.CustomRules)
}

// validateStringValue with detections disabled

func TestValidateStringValue_SQLDetectionDisabled(t *testing.T) {
	config := DefaultInputValidationConfig()
	config.EnableSQLInjectionDetection = false
	err := validateStringValue(config, "search", "SELECT * FROM users")
	// SQL injection detection is disabled, should pass
	assert.NoError(t, err)
}

func TestValidateStringValue_XSSDetectionDisabled(t *testing.T) {
	config := DefaultInputValidationConfig()
	config.EnableXSSDetection = false
	config.EnableSQLInjectionDetection = false
	err := validateStringValue(config, "comment", "<script>alert(1)</script>")
	assert.NoError(t, err)
}

func TestValidateStringValue_PathTraversalDetectionDisabled(t *testing.T) {
	config := DefaultInputValidationConfig()
	config.EnablePathTraversalDetection = false
	config.EnableSQLInjectionDetection = false
	config.EnableXSSDetection = false
	err := validateStringValue(config, "path", "../../../etc/passwd")
	assert.NoError(t, err)
}

// F2: the XSS rules must catch tags that are not closed, span lines, and the
// other active-content elements; benign prose must still pass.
func TestDetectXSS_Round2Corpus(t *testing.T) {
	bad := []string{
		`<script>alert(1)</script>`,
		`<script src=//evil.example/x.js>`,
		`<script src="https://evil.example/x.js"`,
		"<script>\nalert(1)\n</script>",
		"<SCRIPT\n>alert(1)",
		`<iframe src="javascript:alert(1)">`,
		`<iframe src=//evil.example>`,
		`<base href="//evil.example/">`,
		`<link rel=stylesheet href=//evil.example/x.css>`,
		`<style>@import 'https://evil.example/x.css';</style>`,
		`<style>body{background:url(javascript:alert(1))}`,
		`<img src=x onerror=alert(1)>`,
		`<img src=x onerror = alert(1)`,
		`<svg onload=alert(1)>`,
		`<body onload=alert(1)>`,
		`<a href="javascript:alert(1)">x</a>`,
		`<a href="JaVaScRiPt:alert(1)">`,
		`<a href="data:text/html;base64,PHNjcmlwdD4=">`,
		`<object data="x.swf">`,
		`<embed src=x.swf>`,
		`<meta http-equiv="refresh" content="0;url=//evil.example">`,
		`<form action=//evil.example><input type=submit>`,
		`<math><mtext><script>alert(1)`,
		`<div onmouseover="alert(1)">hi</div>`,
		`<details open ontoggle=alert(1)>`,
	}
	for _, in := range bad {
		assert.True(t, DetectXSS(in), "must detect: %q", in)
	}
	good := []string{
		"hello world", "5 < 6 and 7 > 3", "a script for a play", "use the <b> tag sparingly",
		"The javascript language", "onboarding steps", "I prefer base and link layouts",
		"email me at a@b.test", "P4ss<word>Throwaway", "style guide", "iframe is a word",
	}
	for _, in := range good {
		assert.False(t, DetectXSS(in), "must accept: %q", in)
	}
}

// R10 kill test: a quote-tautology without any keyword like UNION/SELECT.
func TestDetectSQLInjection_QuoteTautologies(t *testing.T) {
	for _, in := range []string{`x' OR y`, `x" AND z`, `' OR '1'='1`, `admin'--`, `x' or 1=1 --`} {
		assert.True(t, DetectSQLInjection(in), "must detect: %q", in)
	}
	for _, in := range []string{"Password with OR and AND words", "it's an order or an and", "select a color"} {
		assert.False(t, DetectSQLInjection(in), "must accept: %q", in)
	}
}

// M2: only a TOP-LEVEL secret key is exempt from the SQL/XSS value scan.
func TestValidateJSONValues_SecretExemptionTopLevelOnly(t *testing.T) {
	config := DefaultInputValidationConfig()
	assert.NoError(t, validateJSONValues(config, map[string]interface{}{"password": "Pa'ss; DROP TABLE x OR 1=1"}))
	err := validateJSONValues(config, map[string]interface{}{"meta": map[string]interface{}{"password": "<script>alert(1)</script>"}})
	assert.Error(t, err, "nested password-named key is not exempt")
}

// N3: arrays and objects under a secret-named key are not exempt.
func TestValidateJSONValues_SecretKeyNestedArrayStillValidated(t *testing.T) {
	config := DefaultInputValidationConfig()
	assert.Error(t, validateJSONValues(config, map[string]interface{}{"password": []interface{}{"<script>alert(1)</script>"}}))
	assert.Error(t, validateJSONValues(config, map[string]interface{}{"password": map[string]interface{}{"x": "<script>alert(1)</script>"}}))
	assert.NoError(t, validateJSONValues(config, map[string]interface{}{"password": "Aa1!<b>ok"}))
}
