# err.Error() class scan sources (PX-F18)

Two Go AST programs used by `docs/qa/err_error_class_enumeration_20261113.md`. Stored as `.txt` so no Go tool compiles them.
Use: copy one to an empty scratch directory as `main.go` with a `go.mod` (`module astscantool`, `go 1.22`), `go build`, run `./astscan <dir>` (sink scan, tab-separated: file, line, function, sink, status, kind, expression) or the census build with `<dir>`.
