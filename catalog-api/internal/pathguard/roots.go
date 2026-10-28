package pathguard

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Environment variables that configure the confinement roots. Each is a colon-separated
// list of directories; when set (non-empty) it REPLACES the built-in default of that class.
const (
	// EnvReadRoots: where the server may READ request-named paths (sync sources, upload
	// sources of a sync, conversion inputs, cover-art lookup). Default: MEDIA_ROOT_PATH (if
	// set) plus ./data.
	EnvReadRoots = "CATALOGIZER_READ_ROOTS"
	// EnvSyncRoots: where a sync may WRITE (download targets, destination_directory,
	// bidirectional sides). Everything under these roots is also readable. Default: ./data.
	EnvSyncRoots = "CATALOGIZER_SYNC_ROOTS"
	// EnvConversionRoots: where a conversion may WRITE its output. Everything under these
	// roots is also readable as a conversion input. Default: ./data.
	EnvConversionRoots = "CATALOGIZER_CONVERSION_ROOTS"
	// EnvMediaRoot is the existing media-library variable; it seeds the default read roots.
	EnvMediaRoot = "MEDIA_ROOT_PATH"
)

// DataDir is the built-in default (relative to the working directory).
var DataDir = filepath.Join(".", "data")

// ReadRoots returns the effective read roots (re-read from the environment on every call).
func ReadRoots() []string {
	defaults := []string{DataDir}
	if m := getenv(EnvMediaRoot); m != "" {
		defaults = append([]string{m}, defaults...)
	}
	return RootsFromEnv(EnvReadRoots, defaults...)
}

// SyncWriteRoots returns the effective sync write roots.
func SyncWriteRoots() []string { return RootsFromEnv(EnvSyncRoots, DataDir) }

// ConversionWriteRoots returns the effective conversion write roots.
func ConversionWriteRoots() []string { return RootsFromEnv(EnvConversionRoots, DataDir) }

// Readable returns the roots a path may be READ from: the read roots plus the given write roots.
func Readable(write []string) []string {
	out := append([]string{}, ReadRoots()...)
	return append(out, write...)
}

// Describe renders the effective roots for a single boot log line.
func Describe() string {
	return fmt.Sprintf("read=%s sync_write=%s conversion_write=%s",
		strings.Join(ReadRoots(), ":"), strings.Join(SyncWriteRoots(), ":"), strings.Join(ConversionWriteRoots(), ":"))
}
