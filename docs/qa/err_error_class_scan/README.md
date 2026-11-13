# err.Error() class scan sources (PX-F18)

Three Go AST programs used by `docs/qa/err_error_class_enumeration_20261113.md`. Stored as `.txt` so no Go tool compiles them.
Use: copy one to an empty scratch directory as `main.go` with a `go.mod` (`module astscantool`, `go 1.22`), `go build`, run `./astscan <dir>` (sink scan, tab-separated: file, line, function, sink, status, kind, expression) or the census build with `<dir>`.

`flows_assign.go.txt` (rework 3) is the flow scan: build it the same way and run `./flows <dir>`; it prints tab-separated rows of error-derived text assigned to struct fields, map or kv entries, channel sends and call arguments (it skips `challenges/`). `astscan_sinks.go.txt` was extended in rework 3 with kind `H:` (helper inside a sink argument) and type-based err-like identifiers.
