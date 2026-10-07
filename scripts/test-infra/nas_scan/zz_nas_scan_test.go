package services

// READ-ONLY real-NAS scan harness (overlay file: it is injected into the services package with `go test -overlay`, never copied into the tree).
// It drives the REAL SMBScanner + insertFileRecord against ONE real share through a decorator that (a) allows only ListDirectory/GetFileInfo/Connect/Disconnect/IsConnected/TestConnection/GetProtocol/GetConfig,
// (b) throttles to <= 1 listing per second, (c) refuses every write-class method. Credentials are read from the auth file in NAS_AUTH_FILE (0600); never argv/env/output.
// Output: NAS_OUT (JSON): counts only (rows ingested, entries observed from the wire, per-type), never names.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type roThrottled struct {
	filesystem.FileSystemClient
	mu       sync.Mutex
	last     time.Time
	lists    int
	observed map[string]bool // dir|name -> isDir
}

var errWrite = errors.New("write-class operation refused: read-only NAS policy")

func (r *roThrottled) ListDirectory(ctx context.Context, p string) ([]*filesystem.FileInfo, error) {
	r.mu.Lock()
	if d := time.Second - time.Since(r.last); d > 0 {
		time.Sleep(d)
	}
	r.last = time.Now()
	r.lists++
	r.mu.Unlock()
	out, err := r.FileSystemClient.ListDirectory(ctx, p)
	r.mu.Lock()
	for _, f := range out {
		r.observed[p+"|"+f.Name] = f.IsDir
	}
	r.mu.Unlock()
	return out, err
}
func (r *roThrottled) WriteFile(context.Context, string, io.Reader) error {
	return errWrite
}
func (r *roThrottled) CreateDirectory(context.Context, string) error    { return errWrite }
func (r *roThrottled) DeleteDirectory(context.Context, string) error    { return errWrite }
func (r *roThrottled) DeleteFile(context.Context, string) error         { return errWrite }
func (r *roThrottled) CopyFile(context.Context, string, string) error   { return errWrite }

func readAuth(t *testing.T, p string) (string, string) {
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	var u, pw string
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "username = "); ok {
			u = v
		}
		if v, ok := strings.CutPrefix(l, "password = "); ok {
			pw = v
		}
	}
	require.NotEmpty(t, u)
	require.NotEmpty(t, pw)
	return u, pw
}

func TestNASScanReadOnly(t *testing.T) {
	ip, share, outp := os.Getenv("NAS_IP"), os.Getenv("NAS_SHARE"), os.Getenv("NAS_OUT")
	if ip == "" || share == "" || outp == "" {
		t.Skip("NAS_IP/NAS_SHARE/NAS_OUT not set")
	}
	depth, _ := strconv.Atoi(os.Getenv("NAS_DEPTH"))
	tmo, _ := strconv.Atoi(os.Getenv("NAS_TIMEOUT_S"))
	if tmo == 0 {
		tmo = 240
	}
	user, pw := readAuth(t, os.Getenv("NAS_AUTH_FILE"))
	real := filesystem.NewSmbClient(&filesystem.SmbConfig{Host: ip, Port: 445, Share: share, Username: user, Password: pw})
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(tmo)*time.Second)
	defer cancel()
	require.NoError(t, real.Connect(ctx))
	defer real.Disconnect(context.Background())
	cl := &roThrottled{FileSystemClient: real, observed: map[string]bool{}}

	sqlDB, err := sql.Open("sqlite3", "file:nasscan?mode=memory&cache=shared&_foreign_keys=1")
	require.NoError(t, err)
	defer sqlDB.Close()
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	rootID, err := db.InsertReturningID(context.Background(), `INSERT INTO storage_roots (name, protocol, path, enabled, max_depth) VALUES (?, 'smb', '/', 1, ?)`, "nas-ro-survey", depth)
	require.NoError(t, err)
	job := ScanJob{ID: "nas-ro", StorageRoot: &models.StorageRoot{ID: rootID, Name: "nas-ro-survey", Protocol: "smb"}, Path: ".", MaxDepth: depth, Context: ctx}
	status := &ScanStatus{JobID: job.ID, StartTime: time.Now(), Status: "running"}
	scanErr := NewSMBScanner(db, zap.NewNop()).ScanPath(ctx, cl, job, status)

	var rows, files, dirs, deleted int64
	q := func(dst *int64, s string) { require.NoError(t, db.QueryRowContext(context.Background(), s, rootID).Scan(dst)) }
	q(&rows, `SELECT COUNT(*) FROM files WHERE storage_root_id = ?`)
	q(&files, `SELECT COUNT(*) FROM files WHERE storage_root_id = ? AND is_directory = 0`)
	q(&dirs, `SELECT COUNT(*) FROM files WHERE storage_root_id = ? AND is_directory = 1`)
	q(&deleted, `SELECT COUNT(*) FROM files WHERE storage_root_id = ? AND deleted = 1`)
	var obsFiles, obsDirs int64
	for _, d := range cl.observed {
		if d {
			obsDirs++
		} else {
			obsFiles++
		}
	}
	res := map[string]interface{}{
		"schema": "wp12-nas-scan/1", "share": share, "max_depth": depth, "timeout_s": tmo, "listing_requests": cl.lists,
		"scan_error": func() string {
			if scanErr != nil {
				return scanErr.Error()
			}
			return ""
		}(),
		"db_rows": rows, "db_files": files, "db_dirs": dirs, "db_deleted": deleted,
		"wire_observed_files": obsFiles, "wire_observed_dirs": obsDirs, "wire_observed_total": obsFiles + obsDirs,
		"counts_match_wire": rows == obsFiles+obsDirs, "writes_performed": 0, "names_recorded": false,
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	require.NoError(t, os.WriteFile(outp, b, 0o644))
	require.Greater(t, rows, int64(0), "scanner ingested no rows")
}
