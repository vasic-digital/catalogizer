//go:build cgo && !windows

package database

/*
#include <stdint.h>
#include <time.h>

// sqlite3_busy_handler is defined (SQLITE_API, external linkage) in the
// amalgamation compiled by github.com/mutecomm/go-sqlcipher; it is linked into
// the same binary. The handle is passed as an opaque pointer.
extern int sqlite3_busy_handler(void *db, int (*xBusy)(void *, int), void *pArg);

// catalogizer_busy_cb mirrors SQLite's own sqliteDefaultBusyCallback for the
// HAVE_USLEEP case (same delay table, same accumulated-timeout accounting),
// but sleeps with nanosleep. The bundled driver compiles SQLite without
// HAVE_USLEEP, which makes the stock handler sleep a whole second per retry.
static int catalogizer_busy_cb(void *arg, int count) {
	static const int delays[] = {1, 2, 5, 10, 15, 20, 25, 25, 25, 50, 50, 100};
	static const int totals[] = {0, 1, 3, 8, 18, 33, 53, 78, 103, 128, 178, 228};
	const int ndelay = (int)(sizeof(delays) / sizeof(delays[0]));
	int timeout = (int)(intptr_t)arg;
	int delay, prior;
	struct timespec ts;

	if (count < 0) {
		count = 0;
	}
	if (count < ndelay) {
		delay = delays[count];
		prior = totals[count];
	} else {
		delay = delays[ndelay - 1];
		prior = totals[ndelay - 1] + delay * (count - (ndelay - 1));
	}
	if (prior + delay > timeout) {
		delay = timeout - prior;
		if (delay <= 0) {
			return 0;
		}
	}
	ts.tv_sec = delay / 1000;
	ts.tv_nsec = (long)(delay % 1000) * 1000000L;
	while (nanosleep(&ts, &ts) != 0) {
		// interrupted by a signal: sleep the remainder
	}
	return 1;
}

static int catalogizer_install_busy_handler(void *db, int timeoutMS) {
	return sqlite3_busy_handler(db, catalogizer_busy_cb, (void *)(intptr_t)timeoutMS);
}
*/
import "C"

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"unsafe"

	sqlite3 "github.com/mutecomm/go-sqlcipher"
)

// sqliteBusyHandlerErr is nil when the replacement busy handler was attached
// to the go-sqlcipher driver instance registered as "sqlite3". The hook lives
// on that shared driver instance, so it then applies to every connection that
// driver opens in this process, whichever package calls sql.Open (for example
// internal/media/database in the catalog-api binary). It applies ONLY in
// processes that link catalogizer/database: a test binary of a package that
// does not import it keeps SQLite's stock 1 s handler.
//
// It is non-nil (and the stock handler stays in place) when the driver or its
// SQLiteConn layout is not the one this file was written against;
// TestSQLiteBusyHandler_HookInstalled then fails loudly instead of the fix
// silently lapsing.
//
// Known limits, pinned by TestSQLiteBusyHandler_RuntimePragmaRestoresStockHandler:
//   - sqlite3_busy_handler resets the connection's busyTimeout, so after the
//     hook ran "PRAGMA busy_timeout" reads 0 even though the DSN timeout is
//     still honoured by the replacement handler.
//   - A later "PRAGMA busy_timeout=N" (or sqlite3_busy_timeout) on a
//     connection reinstalls SQLite's stock handler, i.e. 1 s retries, on that
//     connection. No production code issues it; only
//     encryption_round2_test.go (TestHardening_X5_BackupRequiresWriteLock)
//     does, on its own connection.
//   - SQLite's VFS sleep (unixSleep) is untouched and still rounds every sleep
//     up to whole seconds; walTryBeginRead's micro-second back-off after five
//     WAL read retries therefore still costs seconds.
//
// Alternative considered: building the driver with CGO_CFLAGS=-DHAVE_USLEEP=1
// fixes both the busy handler and unixSleep without touching driver internals,
// but go.mod cannot set cgo flags for a dependency, so every build, test and
// container invocation would have to export it, and a missing export silently
// brings the 1 s behaviour back. This hook works for every invocation; the
// flag stays the better long-term fix together with a driver upgrade.
var sqliteBusyHandlerErr = installSQLiteBusyHandlerHook()

// sqliteConnType is the driver connection type whose layout is checked.
var sqliteConnType = reflect.TypeOf(sqlite3.SQLiteConn{})

// checkSQLiteConnLayout verifies that t starts with the field "db" of type
// *_Ctype_struct_sqlite3 (the C sqlite3 handle), as in the go-sqlcipher
// version pinned in go.mod. It takes the type so tests can feed fake layouts.
func checkSQLiteConnLayout(t reflect.Type) error {
	if t == nil || t.Kind() != reflect.Struct {
		return fmt.Errorf("go-sqlcipher SQLiteConn layout changed: %v is not a struct", t)
	}
	f, ok := t.FieldByName("db")
	if !ok {
		return errors.New("go-sqlcipher SQLiteConn layout changed: no field db")
	}
	if f.Offset != 0 {
		return fmt.Errorf("go-sqlcipher SQLiteConn layout changed: field db at offset %d, want 0", f.Offset)
	}
	if f.Type.Kind() != reflect.Ptr || f.Type.Elem().Name() != "_Ctype_struct_sqlite3" {
		return fmt.Errorf("go-sqlcipher SQLiteConn layout changed: field db has type %v, want *_Ctype_struct_sqlite3", f.Type)
	}
	return nil
}

// sqliteConnHandle returns the C sqlite3 handle held by the driver connection.
// The field is unexported, so its layout is verified before it is read.
func sqliteConnHandle(c *sqlite3.SQLiteConn) (unsafe.Pointer, error) {
	if err := checkSQLiteConnLayout(sqliteConnType); err != nil {
		return nil, err
	}
	h := *(*unsafe.Pointer)(unsafe.Pointer(c))
	if h == nil {
		return nil, errors.New("go-sqlcipher connection has no sqlite3 handle")
	}
	return h, nil
}

// connBusyTimeoutMS reads the busy timeout the driver applied from the DSN
// (_busy_timeout, default 5000) so the replacement handler honours it.
func connBusyTimeoutMS(c *sqlite3.SQLiteConn) (int, error) {
	rows, err := c.Query("PRAGMA busy_timeout", nil)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	dest := make([]driver.Value, len(rows.Columns()))
	if err := rows.Next(dest); err != nil {
		if errors.Is(err, io.EOF) {
			return 0, errors.New("PRAGMA busy_timeout returned no row")
		}
		return 0, err
	}
	v, ok := dest[0].(int64)
	if !ok {
		return 0, fmt.Errorf("PRAGMA busy_timeout returned %T", dest[0])
	}
	return int(v), nil
}

// installSQLiteBusyHandler replaces the stock busy handler on one connection.
func installSQLiteBusyHandler(c *sqlite3.SQLiteConn) error {
	timeout, err := connBusyTimeoutMS(c)
	if err != nil {
		return fmt.Errorf("sqlite busy handler: read busy_timeout: %w", err)
	}
	if timeout <= 0 {
		// _busy_timeout=0 means "fail immediately"; keep SQLite's behaviour.
		return nil
	}
	h, err := sqliteConnHandle(c)
	if err != nil {
		return fmt.Errorf("sqlite busy handler: %w", err)
	}
	if rc := C.catalogizer_install_busy_handler(h, C.int(timeout)); rc != 0 {
		return fmt.Errorf("sqlite busy handler: sqlite3_busy_handler rc=%d", int(rc))
	}
	return nil
}

// newBusyHandlerConnectHook returns the ConnectHook: install runs first and
// its error fails the connection open (no silent fallback to the 1 s
// handler); the previously registered hook, if any, runs afterwards.
func newBusyHandlerConnectHook(install, prev func(*sqlite3.SQLiteConn) error) func(*sqlite3.SQLiteConn) error {
	return func(c *sqlite3.SQLiteConn) error {
		if err := install(c); err != nil {
			return err
		}
		if prev != nil {
			return prev(c)
		}
		return nil
	}
}

// installSQLiteBusyHandlerHook attaches installSQLiteBusyHandler to the
// driver instance registered as "sqlite3", chaining any hook already set.
// It runs during package initialisation, before any connection is opened.
func installSQLiteBusyHandlerHook() error {
	db, err := sql.Open("sqlite3", "")
	if err != nil {
		return fmt.Errorf("sqlite busy handler: %w", err)
	}
	drv, ok := db.Driver().(*sqlite3.SQLiteDriver)
	_ = db.Close() // sql.Open connects lazily; nothing was opened
	if !ok {
		return fmt.Errorf("sqlite busy handler: driver %T is not go-sqlcipher", db.Driver())
	}
	if err := checkSQLiteConnLayout(sqliteConnType); err != nil {
		return fmt.Errorf("sqlite busy handler: %w", err)
	}
	drv.ConnectHook = newBusyHandlerConnectHook(installSQLiteBusyHandler, drv.ConnectHook)
	return nil
}
