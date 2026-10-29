//go:build !cgo || windows

package database

// On Windows SQLite's own busy handler already sleeps in milliseconds
// (SQLITE_OS_WIN branch of sqliteDefaultBusyCallback), and without cgo the
// go-sqlcipher driver is not available, so no replacement handler is needed.
var sqliteBusyHandlerErr error
