// Package smartrules implements the closed rule language behind smart
// collections and smart playlists.
//
// A smart container stores a list of rules and evaluates them at READ time
// against media_items, so its content stays live as the catalogue changes.
//
// Safety model: the rule list is parsed and validated against a closed
// catalogue of fields and operators (unknown field, operator, key or a value
// of the wrong type is an error). The SQL that is run is assembled ONLY from
// fixed fragments of that catalogue; every user-supplied value is a bound
// parameter. No user text is ever concatenated into SQL.
package smartrules

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits of the rule language and of one evaluation.
const (
	MaxNodes      = 25       // rules including nested ones
	MaxDepth      = 3        // nesting levels (top level = 1)
	MaxValueBytes = 256      // one string value
	MaxListValues = 50       // entries of an in / not_in / between value
	MaxRulesJSON  = 64 << 10 // size of the stored rule document
	DefaultLimit  = 50
	MaxLimit      = 500 // result cap of one evaluation page
	MaxLastDays   = 36500
)

// ValidationError is returned for every client-correctable problem; handlers
// map it to HTTP 400.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func bad(format string, a ...interface{}) error {
	return &ValidationError{Msg: fmt.Sprintf(format, a...)}
}

// IsValidation reports whether err is (or wraps) a ValidationError.
func IsValidation(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}

type kind int

const (
	kText kind = iota
	kNumber
	kDate
	kBool
	kMediaType
)

type fieldDef struct {
	kind      kind
	userScope bool // evaluated against the viewing user's own data
	integer   bool // backed by an integer column: fractional or out-of-range values are rejected
}

var fields = map[string]fieldDef{
	"title":      {kind: kText},
	"genre":      {kind: kText},
	"language":   {kind: kText},
	"country":    {kind: kText},
	"path":       {kind: kText},
	"quality":    {kind: kText},
	"media_type": {kind: kMediaType},
	"year":       {kind: kNumber, integer: true},
	"rating":     {kind: kNumber},
	"runtime":    {kind: kNumber, integer: true},
	"size":       {kind: kNumber, integer: true},
	"added":      {kind: kDate},
	"updated":    {kind: kDate},
	"tags":       {kind: kText, userScope: true},
	"favorite":   {kind: kBool, userScope: true},
}

var fieldAliases = map[string]string{
	"created_at": "added", "added_at": "added", "date_added": "added",
	"updated_at": "updated", "last_updated": "updated",
	"mediatype": "media_type", "type": "media_type",
	// names the web rule builder offers for the same data
	"is_favorite": "favorite", "file_size": "size", "filesize": "size",
	"duration": "runtime", "date_modified": "updated", "modified_at": "updated",
}

// operatorAliases maps the builder's date wording onto the closed operator set.
var operatorAliases = map[string]string{"after": "greater_than", "before": "less_than"}

// operators allowed per kind.
var opsByKind = map[kind][]string{
	kText:      {"equals", "not_equals", "contains", "not_contains", "starts_with", "ends_with", "in", "not_in"},
	kNumber:    {"equals", "not_equals", "greater_than", "greater_or_equal", "less_than", "less_or_equal", "between", "in", "not_in"},
	kDate:      {"equals", "greater_than", "greater_or_equal", "less_than", "less_or_equal", "between", "in_last_days"},
	kBool:      {"equals", "not_equals"},
	kMediaType: {"equals", "not_equals", "in", "not_in"},
}

// tags only supports substring matching: the stored text is free-form.
var tagOps = []string{"contains", "not_contains"}

// mediaTypeAliases expands the coarse UI names to concrete media_types rows.
var mediaTypeAliases = map[string][]string{
	"video":    {"movie", "tv_show", "tv_season", "tv_episode"},
	"music":    {"music_artist", "music_album", "song"},
	"document": {"book", "comic"},
	"image":    {}, // the catalogue has no image media type: matches nothing
}

var concreteMediaTypes = map[string]bool{
	"movie": true, "tv_show": true, "tv_season": true, "tv_episode": true,
	"music_artist": true, "music_album": true, "song": true,
	"game": true, "software": true, "book": true, "comic": true,
}

// Rule is one validated rule. Value is already normalised for the field kind.
type Rule struct {
	Field       string
	Operator    string
	Value       interface{} // string | float64 | bool | time.Time | []interface{}
	AndWithPrev bool        // connector to the previous sibling (default AND)
	Nested      []Rule
}

// RuleSet is a validated, ready-to-compile rule list.
type RuleSet struct {
	Rules []Rule
	// Raw is the validated original document, compacted, for storage.
	Raw string
}

// wireRule is the accepted JSON shape of one rule. Unknown keys are refused;
// id/label/field_type are tolerated UI metadata and preserved in storage.
type wireRule struct {
	Field       string            `json:"field"`
	Operator    string            `json:"operator"`
	Op          string            `json:"op"`
	Value       json.RawMessage   `json:"value"`
	Condition   string            `json:"condition"`
	NestedRules []json.RawMessage `json:"nested_rules"`
	ID          json.RawMessage   `json:"id"`
	Label       json.RawMessage   `json:"label"`
	FieldType   json.RawMessage   `json:"field_type"`
}

// Parse validates a list of raw rules. An empty list is an error.
func Parse(raws []json.RawMessage) (*RuleSet, error) {
	if len(raws) == 0 {
		return nil, bad("smart_rules must contain at least one rule")
	}
	count := 0
	rules, err := parseList(raws, 1, &count)
	if err != nil {
		return nil, err
	}
	arr := make([]json.RawMessage, len(raws))
	copy(arr, raws)
	doc, err := json.Marshal(arr)
	if err != nil {
		return nil, bad("smart_rules is not valid JSON")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, doc); err != nil {
		return nil, bad("smart_rules is not valid JSON")
	}
	if buf.Len() > MaxRulesJSON {
		return nil, bad("smart_rules is too large (max %d bytes)", MaxRulesJSON)
	}
	return &RuleSet{Rules: rules, Raw: buf.String()}, nil
}

// ParseStored parses a document previously produced by Parse.
func ParseStored(doc string) (*RuleSet, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal([]byte(doc), &raws); err != nil {
		return nil, bad("stored smart_rules is not a JSON array")
	}
	return Parse(raws)
}

func parseList(raws []json.RawMessage, depth int, count *int) ([]Rule, error) {
	if depth > MaxDepth {
		return nil, bad("rules are nested too deeply (max %d levels)", MaxDepth)
	}
	out := make([]Rule, 0, len(raws))
	for i, raw := range raws {
		*count++
		if *count > MaxNodes {
			return nil, bad("too many rules (max %d including nested)", MaxNodes)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var w wireRule
		if err := dec.Decode(&w); err != nil {
			return nil, bad("rule %d is invalid: %v", i+1, jsonErrText(err))
		}
		r, err := buildRule(w, i+1, depth, count)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func jsonErrText(err error) string {
	s := err.Error()
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

func buildRule(w wireRule, pos, depth int, count *int) (Rule, error) {
	name := strings.ToLower(strings.TrimSpace(w.Field))
	if a, ok := fieldAliases[name]; ok {
		name = a
	}
	def, ok := fields[name]
	if !ok {
		return Rule{}, bad("rule %d: unknown field %q", pos, truncate(w.Field))
	}
	op := w.Operator
	if op == "" {
		op = w.Op
	} else if w.Op != "" && w.Op != w.Operator {
		return Rule{}, bad("rule %d: operator and op disagree", pos)
	}
	op = strings.ToLower(strings.TrimSpace(op))
	if def.kind == kDate {
		if canon, ok := operatorAliases[op]; ok {
			op = canon
		}
	}
	allowed := opsByKind[def.kind]
	if name == "tags" {
		allowed = tagOps
	}
	if !contains(allowed, op) {
		return Rule{}, bad("rule %d: operator %q is not allowed for field %q (allowed: %s)", pos, truncate(op), name, strings.Join(allowed, ", "))
	}
	val, err := normaliseValue(def.kind, name, op, w.Value)
	if err == nil && def.integer {
		err = checkIntegers(val)
	}
	if err != nil {
		return Rule{}, bad("rule %d (%s %s): %v", pos, name, op, err)
	}
	and := true
	if c := strings.ToLower(strings.TrimSpace(w.Condition)); c != "" {
		switch c {
		case "and":
		case "or":
			and = false
		default:
			return Rule{}, bad("rule %d: condition must be and/or, got %q", pos, truncate(w.Condition))
		}
	}
	r := Rule{Field: name, Operator: op, Value: val, AndWithPrev: and}
	if len(w.NestedRules) > 0 {
		nested, err := parseList(w.NestedRules, depth+1, count)
		if err != nil {
			return Rule{}, err
		}
		r.Nested = nested
	}
	return r, nil
}

// truncate shortens s for an error message, cutting on a rune boundary so the message stays valid UTF-8.
func truncate(s string) string {
	if len(s) <= 40 {
		return s
	}
	cut := 40
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}

// checkIntegers rejects fractional or out-of-range numbers for integer-backed fields.
func checkIntegers(v interface{}) error {
	switch x := v.(type) {
	case float64:
		if x != math.Trunc(x) || math.Abs(x) > 9e15 {
			return errors.New("value must be a whole number within range for this field")
		}
	case []interface{}:
		for _, e := range x {
			if err := checkIntegers(e); err != nil {
				return err
			}
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func normaliseValue(k kind, field, op string, raw json.RawMessage) (interface{}, error) {
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, errors.New("value is required")
	}
	switch op {
	case "in", "not_in":
		var list []json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil {
			// a single scalar is accepted as a one element list
			list = []json.RawMessage{raw}
		}
		if len(list) == 0 || len(list) > MaxListValues {
			return nil, fmt.Errorf("value must hold 1..%d entries", MaxListValues)
		}
		out := make([]interface{}, 0, len(list))
		for _, e := range list {
			v, err := scalar(k, field, e)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case "between":
		var list []json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil || len(list) != 2 {
			return nil, errors.New("value must be a two element array [low, high]")
		}
		lo, err := scalar(k, field, list[0])
		if err != nil {
			return nil, err
		}
		hi, err := scalar(k, field, list[1])
		if err != nil {
			return nil, err
		}
		return []interface{}{lo, hi}, nil
	case "in_last_days":
		v, err := scalar(kNumber, field, raw)
		if err != nil {
			return nil, err
		}
		f := v.(float64)
		if f != math.Trunc(f) || f < 1 || f > MaxLastDays {
			return nil, fmt.Errorf("value must be a whole number of days 1..%d", MaxLastDays)
		}
		return f, nil
	}
	return scalar(k, field, raw)
}

func scalar(k kind, field string, raw json.RawMessage) (interface{}, error) {
	raw = bytes.TrimSpace(raw)
	switch k {
	case kText:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			var f float64
			if json.Unmarshal(raw, &f) == nil {
				s = strconv.FormatFloat(f, 'f', -1, 64)
			} else {
				return nil, errors.New("value must be a string")
			}
		}
		if s == "" {
			return nil, errors.New("value must not be empty")
		}
		if len(s) > MaxValueBytes {
			return nil, fmt.Errorf("value too long (max %d bytes)", MaxValueBytes)
		}
		if strings.ContainsRune(s, 0) {
			return nil, errors.New("value contains a NUL byte")
		}
		// U+0001..U+0003 are the separators the SQLite JSON-column readers use internally
		if strings.ContainsAny(s, "\x01\x02\x03") {
			return nil, errors.New("value contains a reserved control character")
		}
		if !utf8.ValidString(s) {
			return nil, errors.New("value is not valid UTF-8")
		}
		return s, nil
	case kNumber:
		var f float64
		if err := json.Unmarshal(raw, &f); err != nil {
			var s string
			if json.Unmarshal(raw, &s) != nil {
				return nil, errors.New("value must be a number")
			}
			p, perr := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if perr != nil {
				return nil, errors.New("value must be a number")
			}
			f = p
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, errors.New("value must be a finite number")
		}
		return f, nil
	case kBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			var s string
			if json.Unmarshal(raw, &s) != nil {
				return nil, errors.New("value must be true or false")
			}
			switch strings.ToLower(strings.TrimSpace(s)) {
			case "true":
				b = true
			case "false":
				b = false
			default:
				return nil, errors.New("value must be true or false")
			}
		}
		return b, nil
	case kDate:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, errors.New("value must be a date string (YYYY-MM-DD or RFC 3339)")
		}
		s = strings.TrimSpace(s)
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return dateOnly(t), nil
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.UTC(), nil
		}
		return nil, errors.New("value must be a date string (YYYY-MM-DD or RFC 3339)")
	case kMediaType:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, errors.New("value must be a media type string")
		}
		s = strings.ToLower(strings.TrimSpace(s))
		if concreteMediaTypes[s] {
			return []string{s}, nil
		}
		if exp, ok := mediaTypeAliases[s]; ok {
			return exp, nil
		}
		return nil, fmt.Errorf("unknown media type %q", truncate(s))
	}
	return nil, errors.New("unsupported value")
}

// dateOnly marks a pure date; compile treats Equal on such as a whole day.
type dateOnly time.Time

// --- compilation ----------------------------------------------------------

// Options control SQL generation.
type Options struct {
	Postgres bool
	// UserID is the viewing user, required by user-scoped fields (tags, favorite).
	UserID int64
	// Now is the clock used by in_last_days; zero means time.Now().
	Now time.Time
	// Timeout bounds one Evaluate/CountUpTo call; zero means DefaultTimeout.
	Timeout time.Duration
}

// DefaultTimeout bounds a single rule evaluation (all queries of one Evaluate call together).
const DefaultTimeout = 10 * time.Second

func (o Options) bound(ctx context.Context) (context.Context, context.CancelFunc) {
	d := o.Timeout
	if d <= 0 {
		d = DefaultTimeout
	}
	return context.WithTimeout(ctx, d)
}

// Compile turns a rule set into a parameterised WHERE fragment over the
// aliases mi (media_items) and mt (media_types).
func Compile(rs *RuleSet, opt Options) (string, []interface{}, error) {
	if rs == nil || len(rs.Rules) == 0 {
		return "", nil, bad("smart_rules must contain at least one rule")
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	c := &compiler{opt: opt}
	sql, err := c.sequence(rs.Rules)
	if err != nil {
		return "", nil, err
	}
	return sql, c.args, nil
}

type compiler struct {
	opt  Options
	args []interface{}
}

// sequence combines siblings: AND binds tighter than OR.
func (c *compiler) sequence(rules []Rule) (string, error) {
	var ors []string
	var ands []string
	flush := func() {
		if len(ands) > 0 {
			ors = append(ors, "("+strings.Join(ands, " AND ")+")")
			ands = nil
		}
	}
	for i, r := range rules {
		if i > 0 && !r.AndWithPrev {
			flush()
		}
		s, err := c.node(r)
		if err != nil {
			return "", err
		}
		ands = append(ands, s)
	}
	flush()
	return "(" + strings.Join(ors, " OR ") + ")", nil
}

func (c *compiler) node(r Rule) (string, error) {
	pred, err := c.predicate(r)
	if err != nil {
		return "", err
	}
	if len(r.Nested) == 0 {
		return pred, nil
	}
	nested, err := c.sequence(r.Nested)
	if err != nil {
		return "", err
	}
	return "(" + pred + " AND " + nested + ")", nil
}

const (
	sizeExpr = "(SELECT COALESCE(SUM(f.size), 0) FROM media_files mf JOIN files f ON f.id = mf.file_id WHERE mf.media_item_id = mi.id AND NOT f.deleted)"
)

func (c *compiler) predicate(r Rule) (string, error) {
	def := fields[r.Field]
	if def.userScope && c.opt.UserID <= 0 {
		return "", bad("field %q needs an authenticated user", r.Field)
	}
	switch r.Field {
	case "path":
		inner, negate, err := c.text("f.path", r)
		if err != nil {
			return "", err
		}
		return existsWrap("SELECT 1 FROM media_files mf JOIN files f ON f.id = mf.file_id WHERE mf.media_item_id = mi.id AND NOT f.deleted AND ", inner, negate), nil
	case "quality":
		inner, negate, err := c.text(c.qualityExpr(), r)
		if err != nil {
			return "", err
		}
		return existsWrap("SELECT 1 FROM media_files mf WHERE mf.media_item_id = mi.id AND ", inner, negate), nil
	case "tags":
		inner, negate, err := c.arrayText("um.tags", r)
		if err != nil {
			return "", err
		}
		// the user id is bound first inside the subquery, before the text args
		return c.userExists("um.tags", inner, negate, r)
	case "favorite":
		return c.favorite(r)
	case "media_type":
		return c.mediaType(r)
	case "title":
		s, _, err := c.textDirect("mi.title", r)
		return s, err
	case "genre":
		return c.genre(r)
	case "language":
		s, _, err := c.textDirect("mi.language", r)
		return s, err
	case "country":
		s, _, err := c.textDirect("mi.country", r)
		return s, err
	case "year":
		return c.number("mi.year", r)
	case "rating":
		return c.number("mi.rating", r)
	case "runtime":
		return c.number("mi.runtime", r)
	case "size":
		return c.number(sizeExpr, r)
	case "added":
		return c.date("mi.first_detected", r)
	case "updated":
		return c.date("mi.last_updated", r)
	}
	return "", bad("unsupported field %q", r.Field)
}

func existsWrap(prefix, inner string, negate bool) string {
	s := "EXISTS (" + prefix + inner + ")"
	if negate {
		return "NOT " + s
	}
	return s
}

// textDirect compiles a text predicate on a column of mi itself. Negative
// operators also match rows where the column is NULL.
func (c *compiler) textDirect(col string, r Rule) (string, bool, error) {
	inner, negate, err := c.text(col, r)
	if err != nil {
		return "", false, err
	}
	if negate {
		return "(" + col + " IS NULL OR NOT " + inner + ")", true, nil
	}
	return inner, false, nil
}

// text returns the POSITIVE predicate and whether the operator is negated.
func (c *compiler) text(col string, r Rule) (string, bool, error) {
	like := func(pattern string) string {
		c.args = append(c.args, pattern)
		return "LOWER(" + col + ") LIKE LOWER(?) ESCAPE '\\'"
	}
	switch r.Operator {
	case "equals", "not_equals":
		c.args = append(c.args, r.Value.(string))
		return "LOWER(" + col + ") = LOWER(?)", r.Operator == "not_equals", nil
	case "contains", "not_contains":
		return like("%" + escapeLike(r.Value.(string)) + "%"), r.Operator == "not_contains", nil
	case "starts_with":
		return like(escapeLike(r.Value.(string)) + "%"), false, nil
	case "ends_with":
		return like("%" + escapeLike(r.Value.(string))), false, nil
	case "in", "not_in":
		list := r.Value.([]interface{})
		ph := make([]string, len(list))
		for i, v := range list {
			ph[i] = "LOWER(?)"
			c.args = append(c.args, v.(string))
		}
		return "LOWER(" + col + ") IN (" + strings.Join(ph, ", ") + ")", r.Operator == "not_in", nil
	}
	return "", false, bad("operator %q not supported for text", r.Operator)
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	return strings.ReplaceAll(s, `_`, `\_`)
}

// userExists wraps a text predicate over user_metadata (per-user data).
func (c *compiler) userExists(col, inner string, negate bool, r Rule) (string, error) {
	// args order: the placeholders appear user_id first, so splice it before the
	// text args that text() already appended for this rule.
	n := countPlaceholders(inner)
	textArgs := append([]interface{}{}, c.args[len(c.args)-n:]...)
	c.args = append(c.args[:len(c.args)-n], c.opt.UserID)
	c.args = append(c.args, textArgs...)
	s := "EXISTS (SELECT 1 FROM user_metadata um WHERE um.media_item_id = mi.id AND um.user_id = ? AND " + inner + ")"
	if negate {
		s = "NOT " + s
	}
	return s, nil
}

func countPlaceholders(s string) int { return strings.Count(s, "?") }

// favoriteEntityTypes is the closed set of entity types the favourites API accepts for media entities
// (handlers.validEntityTypes). The UI stores a catalogue media id under one of them (it defaults to
// "movie"), so the rule matches any of them by media id; a favourite of another kind (a playlist, for
// example) with the same numeric id does not count.
var favoriteEntityTypes = []string{"movie", "tv_show", "tv_season", "tv_episode", "music_artist", "music_album", "song", "game", "software", "book", "comic"}

// favorite reads the canonical favourite store: the `favorites` table, the one the web UI toggle
// (POST/DELETE /favorites) and the Favorites page use, scoped to the viewing user. user_metadata.favorite
// and media_items.is_favorite are legacy flags and are NOT consulted.
func (c *compiler) favorite(r Rule) (string, error) {
	want := r.Value.(bool)
	if r.Operator == "not_equals" {
		want = !want
	}
	ph := make([]string, len(favoriteEntityTypes))
	c.args = append(c.args, c.opt.UserID)
	for i, t := range favoriteEntityTypes {
		ph[i] = "?"
		c.args = append(c.args, t)
	}
	s := "EXISTS (SELECT 1 FROM favorites fv WHERE fv.entity_id = mi.id AND fv.user_id = ? AND fv.entity_type IN (" + strings.Join(ph, ", ") + "))"
	if want {
		return s, nil
	}
	return "NOT " + s, nil
}

// The SQLite build used here (go-sqlcipher) has no JSON1 extension, so JSON-encoded text columns are
// read with plain string functions. This is sound because both columns are written by encoding/json
// (compact, no spaces): every double quote that is syntax is unescaped, every double quote inside a value
// is escaped as \" and every backslash as \\. Protecting the two escapes first (CHAR(2), CHAR(3)) makes the
// remaining quotes unambiguous syntax; \u0026, \u003c and \u003e (Go escapes & < > in JSON) are decoded
// back. Other control-character escapes are not decoded (values containing control characters never
// match; Parse rejects the three reserved separator characters in rule values).
const (
	sqlSep = "CHAR(1)"
)

// protectEscapes lower-cases col and replaces the JSON escapes \\ and \" by sentinels.
func protectEscapes(col string) string {
	return "REPLACE(REPLACE(LOWER(" + col + "), '\\\\', CHAR(2)), '\\\"', CHAR(3))"
}

// decodeJSONText turns the protected body back into plain text.
func decodeJSONText(expr string) string {
	e := "REPLACE(" + expr + ", CHAR(3), '\"')"
	e = "REPLACE(" + e + ", '\\u0026', '&')"
	e = "REPLACE(" + e + ", '\\u003c', '<')"
	e = "REPLACE(" + e + ", '\\u003e', '>')"
	return "REPLACE(" + e + ", CHAR(2), '\\')"
}

// jsonArrayBody is the SQLite expression for a text column holding a JSON array of strings, decoded to
// its elements joined by CHAR(1), lower-cased. A legacy plain-text value is a single element, a JSON null
// or an empty array has no element, NULL stays NULL.
func jsonArrayBody(col string) string {
	s2 := protectEscapes(col)
	arr := decodeJSONText("REPLACE(SUBSTR(" + s2 + ", 3, LENGTH(" + s2 + ") - 4), '\",\"', " + sqlSep + ")")
	return "(CASE WHEN " + col + " IS NULL THEN NULL WHEN SUBSTR(" + s2 + ", 1, 2) = '[\"' AND SUBSTR(" + s2 + ", -2) = '\"]' THEN " + arr +
		" WHEN SUBSTR(" + s2 + ", 1, 2) = '[]' OR LOWER(" + col + ") = 'null' THEN '' ELSE LOWER(" + col + ") END)"
}

// qualityExpr extracts quality_profile (lower-cased) from media_files.quality_info, a QualityInfo JSON
// object written by the analyzer ("1080p", "4K/UHD", ...). Anything else yields NULL.
func (c *compiler) qualityExpr() string {
	if c.opt.Postgres {
		return "(CASE WHEN mf.quality_info ~ '^\\s*[{]' THEN mf.quality_info::jsonb ->> 'quality_profile' END)"
	}
	const key = "'\"quality_profile\":\"'"
	s2 := protectEscapes("mf.quality_info")
	// Only the TOP-LEVEL key counts (Postgres ->> reads top level only). The text before the first
	// occurrence must not hold an unclosed nested object: openers (the three characters ":{ ) minus
	// closers (}) must be zero. A '}' inside an earlier string value makes the rule not match (safe
	// direction); the QualityInfo writer's only nested object is resolution with integer members.
	pre := "SUBSTR(z.s, 1, INSTR(z.s, " + key + ") - 1)"
	depthOK := "(LENGTH(y.pre) - LENGTH(REPLACE(y.pre, '\":{', ''))) / 3 = LENGTH(y.pre) - LENGTH(REPLACE(y.pre, '}', ''))"
	inner := "(SELECT SUBSTR(x.r, 1, INSTR(x.r, '\"') - 1) FROM (SELECT SUBSTR(y.s, y.p + 19) AS r FROM (SELECT z.s AS s, " + pre + " AS pre, INSTR(z.s, " + key + ") AS p FROM (SELECT " + s2 + " AS s) z) y WHERE y.p > 0 AND " + depthOK + ") x)"
	return decodeJSONText(inner)
}

// genre compiles a text predicate over the ELEMENTS of media_items.genre, which the repository stores as
// a JSON array of strings (["Action","Sci-Fi"]). The positive operators mean "some element matches";
// the negative ones mean "no element matches" (so an item without genres matches them).
func (c *compiler) genre(r Rule) (string, error) {
	pos, negate, err := c.arrayText("mi.genre", r)
	if err != nil {
		return "", err
	}
	if c.opt.Postgres {
		// pos is an EXISTS (...) here: a non-null boolean. COALESCE(boolean, 0) is a type error on Postgres.
		if negate {
			return "NOT " + pos, nil
		}
		return pos, nil
	}
	if negate {
		return "NOT COALESCE(" + pos + ", 0)", nil
	}
	return "COALESCE(" + pos + ", 0)", nil
}

// arrayText returns the POSITIVE predicate ("some element of the JSON-array column matches") and whether
// the operator is negated. Elements are compared whole for equals/in, as substrings for contains, and at
// the element boundary for starts_with/ends_with, so JSON syntax characters ([ ] , ") are never content.
func (c *compiler) arrayText(col string, r Rule) (string, bool, error) {
	if c.opt.Postgres {
		inner, negate, err := c.text("je.value", r)
		if err != nil {
			return "", false, err
		}
		src := "jsonb_array_elements_text(CASE WHEN " + col + " = 'null' THEN '[]'::jsonb WHEN " + col + " ~ '^\\s*[[]' THEN " + col + "::jsonb ELSE jsonb_build_array(" + col + ") END) AS je(value)"
		return "EXISTS (SELECT 1 FROM " + src + " WHERE " + inner + ")", negate, nil
	}
	body := jsonArrayBody(col)
	one := func(format string, v string) string {
		c.args = append(c.args, v)
		return strings.ReplaceAll(strings.ReplaceAll(format, "B", body), "?", "LOWER(?)")
	}
	switch r.Operator {
	case "equals", "not_equals":
		return one("INSTR(CHAR(1) || B || CHAR(1), CHAR(1) || ? || CHAR(1)) > 0", r.Value.(string)), r.Operator == "not_equals", nil
	case "in", "not_in":
		var parts []string
		for _, v := range r.Value.([]interface{}) {
			parts = append(parts, one("INSTR(CHAR(1) || B || CHAR(1), CHAR(1) || ? || CHAR(1)) > 0", v.(string)))
		}
		return "(" + strings.Join(parts, " OR ") + ")", r.Operator == "not_in", nil
	case "contains", "not_contains":
		return one("INSTR(B, ?) > 0", r.Value.(string)), r.Operator == "not_contains", nil
	case "starts_with":
		return one("INSTR(CHAR(1) || B, CHAR(1) || ?) > 0", r.Value.(string)), false, nil
	case "ends_with":
		return one("INSTR(B || CHAR(1), ? || CHAR(1)) > 0", r.Value.(string)), false, nil
	}
	return "", false, bad("operator %q not supported for text", r.Operator)
}

func (c *compiler) mediaType(r Rule) (string, error) {
	var names []string
	switch r.Operator {
	case "equals", "not_equals":
		names = r.Value.([]string)
	default:
		for _, v := range r.Value.([]interface{}) {
			names = append(names, v.([]string)...)
		}
	}
	neg := r.Operator == "not_equals" || r.Operator == "not_in"
	if len(names) == 0 {
		if neg {
			return "(1 = 1)", nil
		}
		return "(1 = 0)", nil
	}
	ph := make([]string, len(names))
	for i, n := range names {
		ph[i] = "?"
		c.args = append(c.args, n)
	}
	op := "IN"
	if neg {
		op = "NOT IN"
	}
	return "(mt.name " + op + " (" + strings.Join(ph, ", ") + "))", nil
}

func (c *compiler) number(col string, r Rule) (string, error) {
	cmp := map[string]string{"equals": "=", "not_equals": "<>", "greater_than": ">", "greater_or_equal": ">=", "less_than": "<", "less_or_equal": "<="}
	switch r.Operator {
	case "equals", "greater_than", "greater_or_equal", "less_than", "less_or_equal":
		c.args = append(c.args, r.Value.(float64))
		return "(" + col + " " + cmp[r.Operator] + " ?)", nil
	case "not_equals":
		c.args = append(c.args, r.Value.(float64))
		return "(" + col + " IS NULL OR " + col + " <> ?)", nil
	case "between":
		l := r.Value.([]interface{})
		c.args = append(c.args, l[0].(float64), l[1].(float64))
		return "(" + col + " BETWEEN ? AND ?)", nil
	case "in", "not_in":
		l := r.Value.([]interface{})
		ph := make([]string, len(l))
		for i, v := range l {
			ph[i] = "?"
			c.args = append(c.args, v.(float64))
		}
		if r.Operator == "in" {
			return "(" + col + " IN (" + strings.Join(ph, ", ") + "))", nil
		}
		return "(" + col + " IS NULL OR " + col + " NOT IN (" + strings.Join(ph, ", ") + "))", nil
	}
	return "", bad("operator %q not supported for numbers", r.Operator)
}

func (c *compiler) ts(v interface{}) interface{} {
	var t time.Time
	switch x := v.(type) {
	case dateOnly:
		t = time.Time(x)
	case time.Time:
		t = x
	}
	t = t.UTC()
	if c.opt.Postgres {
		return t
	}
	return t.Format("2006-01-02 15:04:05")
}

func (c *compiler) date(col string, r Rule) (string, error) {
	if !c.opt.Postgres {
		// the writer stores time.Time with the host's zone offset; datetime() normalises it to UTC so the
		// comparison is between instants, not text.
		col = "datetime(" + col + ")"
	}
	switch r.Operator {
	case "equals":
		d, ok := r.Value.(dateOnly)
		if !ok {
			c.args = append(c.args, c.ts(r.Value))
			return "(" + col + " = ?)", nil
		}
		start := time.Time(d)
		c.args = append(c.args, c.ts(start), c.ts(start.Add(24*time.Hour)))
		return "(" + col + " >= ? AND " + col + " < ?)", nil
	case "greater_than":
		// a pure date means "after that whole day"
		v := r.Value
		if d, ok := v.(dateOnly); ok {
			v = time.Time(d).Add(24 * time.Hour)
			c.args = append(c.args, c.ts(v))
			return "(" + col + " >= ?)", nil
		}
		c.args = append(c.args, c.ts(v))
		return "(" + col + " > ?)", nil
	case "greater_or_equal":
		c.args = append(c.args, c.ts(r.Value))
		return "(" + col + " >= ?)", nil
	case "less_than":
		c.args = append(c.args, c.ts(r.Value))
		return "(" + col + " < ?)", nil
	case "less_or_equal":
		v := r.Value
		if d, ok := v.(dateOnly); ok {
			c.args = append(c.args, c.ts(time.Time(d).Add(24*time.Hour)))
			return "(" + col + " < ?)", nil
		}
		c.args = append(c.args, c.ts(v))
		return "(" + col + " <= ?)", nil
	case "between":
		l := r.Value.([]interface{})
		hi := l[1]
		if d, ok := hi.(dateOnly); ok {
			c.args = append(c.args, c.ts(l[0]), c.ts(time.Time(d).Add(24*time.Hour)))
			return "(" + col + " >= ? AND " + col + " < ?)", nil
		}
		c.args = append(c.args, c.ts(l[0]), c.ts(hi))
		return "(" + col + " BETWEEN ? AND ?)", nil
	case "in_last_days":
		days := r.Value.(float64)
		from := c.opt.Now.UTC().Add(-time.Duration(days) * 24 * time.Hour)
		c.args = append(c.args, c.ts(from))
		return "(" + col + " >= ?)", nil
	}
	return "", bad("operator %q not supported for dates", r.Operator)
}

// --- evaluation -----------------------------------------------------------

// Querier is satisfied by *database.DB and *sql.DB.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Item is one evaluated media item.
type Item struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	MediaType string     `json:"media_type"`
	Year      *int       `json:"year,omitempty"`
	Rating    *float64   `json:"rating,omitempty"`
	Genre     *string    `json:"genre,omitempty"`
	AddedAt   *time.Time `json:"added_at,omitempty"`
}

// NormalizePage clamps limit/offset to the evaluation caps.
func NormalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

const fromClause = " FROM media_items mi JOIN media_types mt ON mt.id = mi.media_type_id WHERE "

// CountUpTo counts the matches but stops after max+1 rows, so the cost is bounded however large the
// catalogue is. capped reports that more than max items match (n == max then).
func CountUpTo(ctx context.Context, q Querier, rs *RuleSet, opt Options, max int) (n int, capped bool, err error) {
	if max < 1 {
		max = 1
	}
	ctx, cancel := opt.bound(ctx)
	defer cancel()
	where, args, err := Compile(rs, opt)
	if err != nil {
		return 0, false, err
	}
	var got int
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM (SELECT 1"+fromClause+where+" LIMIT ?) c", append(append([]interface{}{}, args...), max+1)...).Scan(&got); err != nil {
		return 0, false, fmt.Errorf("smart rules count: %w", err)
	}
	if got > max {
		return max, true, nil
	}
	return got, false, nil
}

// Evaluate runs the rule set and returns one page plus the total match count.
// The page is fully read and the cursor closed before returning.
func Evaluate(ctx context.Context, q Querier, rs *RuleSet, opt Options, limit, offset int) ([]Item, int, error) {
	limit, offset = NormalizePage(limit, offset)
	ctx, cancel := opt.bound(ctx)
	defer cancel()
	where, args, err := Compile(rs, opt)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*)"+fromClause+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("smart rules count: %w", err)
	}
	pageArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := q.QueryContext(ctx,
		"SELECT mi.id, mi.title, mt.name, mi.year, mi.rating, mi.genre, mi.first_detected"+fromClause+where+" ORDER BY mi.id LIMIT ? OFFSET ?",
		pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("smart rules query: %w", err)
	}
	defer rows.Close()
	items := make([]Item, 0, limit)
	for rows.Next() {
		var (
			it     Item
			year   sql.NullInt64
			rating sql.NullFloat64
			genre  sql.NullString
			added  sql.NullTime
		)
		if err := rows.Scan(&it.ID, &it.Title, &it.MediaType, &year, &rating, &genre, &added); err != nil {
			return nil, 0, fmt.Errorf("smart rules scan: %w", err)
		}
		if year.Valid {
			y := int(year.Int64)
			it.Year = &y
		}
		if rating.Valid {
			it.Rating = &rating.Float64
		}
		if genre.Valid {
			it.Genre = &genre.String
		}
		if added.Valid {
			t := added.Time
			it.AddedAt = &t
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("smart rules rows: %w", err)
	}
	return items, total, nil
}
