package errleak

import (
	"os"
	"testing"
)

const impSvc = "import \"catalogizer/internal/services\"\n"

// identityCase is one fixture for the callee-identity class (ERRLEAK-FU-C, ERRLEAK-FU-D). want is the number of
// hits Sweep must report: a look-alike of a sanctioned mapper/wrapper must NOT be sanctioned (want>=1), the real
// one still is (want==0).
type identityCase struct {
	src  string
	want int
}

func fnSrc(imports, body string) string {
	return "package p\n" + imports + "func f(c *gin.Context, err error, w http.ResponseWriter) {\n" + body + "}\n"
}

func identityCases() map[string]identityCase {
	return map[string]identityCase{
		// --- golden-bad (must be flagged) ---
		"S7_closure_shadows_mapper":     {fnSrc("", "\tvalidationText := func(e error) string { return e.Error() }\n\tc.JSON(400, gin.H{\"error\": validationText(err)})\n"), 1},
		"S7_var_func_shadows_mapper":    {fnSrc("", "\tvar validationText = func(e error) string { return e.Error() }\n\tc.JSON(400, gin.H{\"error\": validationText(err)})\n"), 1},
		"S7_string_var_shadows_mapper":  {fnSrc("", "\tvalidationText := err.Error()\n\tc.JSON(400, gin.H{\"error\": validationText})\n"), 1},
		"S7_param_shadows_mapper":       {"package p\nfunc f(c *gin.Context, err error, validationText func(error) string) {\n\tc.JSON(400, gin.H{\"error\": validationText(err)})\n}\n", 1},
		"S7_range_var_shadows_mapper":   {fnSrc("", "\tfor _, validationText := range fs {\n\t\tc.JSON(400, gin.H{\"error\": validationText(err)})\n\t}\n"), 1},
		"S7_namedresult_shadows_mapper": {"package p\nfunc f(c *gin.Context, err error) (validationText func(error) string) {\n\tc.JSON(400, gin.H{\"error\": validationText(err)})\n\treturn\n}\n", 1},
		"S7_pkgvar_shadows_mapper":      {"package p\nvar validationText = func(e error) string { return e.Error() }\n" + fnSrc("", "\tc.JSON(400, gin.H{\"error\": validationText(err)})\n")[len("package p\n"):], 1},
		"S7_shadows_safe_wrapper":       {fnSrc("", "\twriteJobError := func(c *gin.Context, e error, m string) { stash = e.Error() }\n\twriteJobError(c, err, \"x\")\n"), 1},
		"S7_param_shadows_wrapper":      {"package p\nfunc f(c *gin.Context, err error, writeJobError func(*gin.Context, error, string)) {\n\twriteJobError(c, err, \"x\")\n}\n", 1},
		"D_method_named_like_mapper":    {fnSrc("", "\tc.JSON(400, gin.H{\"error\": other.validationText(err)})\n"), 1},
		"D_method_named_like_wrapper":   {fnSrc("", "\tother.writeJobError(c, err, \"x\")\n"), 1},
		"D_selfmethod_like_wrapper":     {fnSrc("", "\th.writeFixedLogError(w, r, err, 500)\n"), 1},
		"D_wrong_pkg_scrub":             {fnSrc("import \"evil/scrub\"\n", "\tc.JSON(500, gin.H{\"error\": scrub.ScrubErrorFor(err, root)})\n"), 1},
		"D_unimported_pkg_scrub":        {fnSrc("", "\tc.JSON(500, gin.H{\"error\": services.ScrubErrorFor(err, root)})\n"), 1},
		"D_aliased_other_pkg_scrub":     {fnSrc("import services \"evil/other\"\n", "\tc.JSON(500, gin.H{\"error\": services.ScrubErrorFor(err, root)})\n"), 1},
		"D_local_var_named_services":    {fnSrc(impSvc, "\tservices := obj\n\tc.JSON(500, gin.H{\"error\": services.ScrubErrorFor(err, root)})\n"), 1},
		// --- golden-good (must stay clean) ---
		"G_real_mapper":             {fnSrc("", "\tc.JSON(400, gin.H{\"error\": \"bad: \" + validationText(err)})\n"), 0},
		"G_real_wrapper":            {fnSrc("", "\twriteJobError(c, err, \"x\")\n"), 0},
		"G_real_wrapper_sync":       {fnSrc("", "\tif respondSyncClassError(c, err) {\n\t\treturn\n\t}\n"), 0},
		"G_real_scrub":              {fnSrc(impSvc, "\tc.JSON(500, gin.H{\"error\": services.ScrubErrorFor(err, root)})\n"), 0},
		"G_real_scrub_alias":        {fnSrc("import svc \"catalogizer/internal/services\"\n", "\tc.JSON(500, gin.H{\"error\": svc.ScrubErrorFor(err, root)})\n"), 0},
		"G_unrelated_shadow_other":  {fnSrc("", "\tother := func(e error) string { return \"fixed\" }\n\t_ = other\n\tc.JSON(400, gin.H{\"error\": validationText(err)})\n"), 0},
		"G_shadow_in_other_scope":   {fnSrc("", "\t{\n\t\tvalidationText := func(e error) string { return \"fixed\" }\n\t\t_ = validationText\n\t}\n\tc.JSON(400, gin.H{\"error\": validationText(err)})\n"), 0},
		"G_toplevel_func_same_file": {"package p\nfunc validationText(e error) string { return \"fixed\" }\n" + fnSrc("", "\tc.JSON(400, gin.H{\"error\": validationText(err)})\n")[len("package p\n"):], 0},
	}
}

func TestCalleeIdentityIsResolved(t *testing.T) {
	for name, c := range identityCases() {
		if got := hits(t, c.src); len(got) != c.want {
			t.Errorf("%s: want %d hits, got %v", name, c.want, got)
		}
	}
}

// TestCalleeIdentityControlNeedle: the same frame with a genuine leak is still seen, so a zero above is not a
// blind instrument.
func TestCalleeIdentityControlNeedle(t *testing.T) {
	if got := hits(t, fnSrc("", "\tc.JSON(500, gin.H{\"error\": err.Error()})\n")); len(got) != 1 {
		t.Fatalf("needle not seen: %v", got)
	}
}

// TestCalleeIdentityOnRealFiles (control needle on the real tree): real handler files stay clean with their real
// mappers, and an injected raw leak or an injected shadowing look-alike appended to the SAME real source is flagged.
func TestCalleeIdentityOnRealFiles(t *testing.T) {
	for _, rel := range []string{"../../handlers/scan_handler.go", "../../handlers/collection_handler.go"} {
		b, err := os.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		base, err := Sweep(rel, src)
		if err != nil {
			t.Fatal(err)
		}
		leak := "\nfunc injectedLeak(c *gin.Context, err error) {\n\tc.JSON(500, gin.H{\"error\": err.Error()})\n}\n"
		if got, _ := Sweep(rel, src+leak); len(got) != len(base)+1 {
			t.Errorf("%s: injected raw leak not flagged (base %d, got %d)", rel, len(base), len(got))
		}
		shadow := "\nfunc injectedShadow(c *gin.Context, err error) {\n\tvalidationText := func(e error) string { return e.Error() }\n\tc.JSON(400, gin.H{\"error\": validationText(err)})\n}\n"
		if got, _ := Sweep(rel, src+shadow); len(got) != len(base)+1 {
			t.Errorf("%s: injected shadowing mapper not flagged (base %d, got %d)", rel, len(base), len(got))
		}
	}
}
