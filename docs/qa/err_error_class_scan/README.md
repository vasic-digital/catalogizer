# err.Error() class scan sources (PX-F18)

Three Go AST programs used by `docs/qa/err_error_class_enumeration_20261113.md`. Stored as `.txt` so no Go tool compiles them.
Use: copy one to an empty scratch directory as `main.go` with a `go.mod` (`module astscantool`, `go 1.22`), `go build`, run `./astscan <dir>` (sink scan, tab-separated: file, line, function, sink, status, kind, expression) or the census build with `<dir>`.

`flows_assign.go.txt` (rework 3) is the flow scan: build it the same way and run `./flows <dir>`; it prints tab-separated rows of error-derived text assigned to struct fields, map or kv entries, channel sends and call arguments (it skips `challenges/`). `astscan_sinks.go.txt` was extended in rework 3 with kind `H:` (helper inside a sink argument) and type-based err-like identifiers.

`typed_scan_gopackages.go.txt` (PXF18DOC-F5) is the typed `go/packages` scan used by the enumeration doc (169 rows at 713a4f380): build it in a scratch module that requires golang.org/x/tools (v0.52.0) and run `./typedscan <catalog-api dir>`; it prints tab-separated `file line callee argument errname|NOT-errname` rows plus `ERRSAMPLE`/`pkg errors` lines for packages that fail to type-check (submodules absent).
