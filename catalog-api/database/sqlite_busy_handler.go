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

// sqliteBusyHandlerErr is nil when every connection opened through the
// "sqlite3" driver in this process gets the millisecond-granularity busy
// handler. It is non-nil (and the stock 1 s handler stays in place) when the
// driver's connection layout is not the one this file was written against;
// TestSQLiteBusyHandler_* then fail loudly instead of the fix silently lapsing.
var sqliteBusyHandlerErr = installSQLiteBusyHandlerHook()

// sqliteConnHandle returns the *C.sqlite3 held by the driver connection. The
// field is unexported, so its layout is verified before it is read.
func sqliteConnHandle(c *sqlite3.SQLiteConn) (unsafe.Pointer, error) {
	if err := checkSQLiteConnLayout(); err != nil {
		return nil, err
	}
	h := *(*unsafe.Pointer)(unsafe.Pointer(c))
	if h == nil {
		return nil, errors.New("go-sqlcipher connection has no sqlite3 handle")
	}
	return h, nil
}

// checkSQLiteConnLayout verifies that SQLiteConn starts with the pointer field
// "db" (the *C.sqlite3 handle), as in the go-sqlcipher version pinned in go.mod.
func checkSQLiteConnLayout() error {
	f, ok := reflect.TypeOf(sqlite3.SQLiteConn{}).FieldByName("db")
	if !ok || f.Offset != 0 || f.Type.Kind() != reflect.Ptr {
		return errors.New("go-sqlcipher SQLiteConn layout changed: field db not a pointer at offset 0")
	}
	return nil
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
	if err := checkSQLiteConnLayout(); err != nil {
		return fmt.Errorf("sqlite busy handler: %w", err)
	}
	prev := drv.ConnectHook
	drv.ConnectHook = func(c *sqlite3.SQLiteConn) error {
		if err := installSQLiteBusyHandler(c); err != nil {
			return err
		}
		if prev != nil {
			return prev(c)
		}
		return nil
	}
	return nil
}
