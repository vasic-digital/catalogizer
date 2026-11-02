package database

import (
	"path/filepath"
	"testing"

	"catalogizer/config"

	"github.com/stretchr/testify/assert"
)

func TestCAT001_DataDir(t *testing.T) {
	dir := t.TempDir()
	assert.Equal(t, dir, (&DB{config: &config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "c.db")}}).DataDir())
	assert.Equal(t, dir, (&DB{config: &config.DatabaseConfig{Path: filepath.Join(dir, "c.db")}}).DataDir(), "empty type defaults to sqlite")
	assert.Empty(t, (&DB{config: &config.DatabaseConfig{Type: "sqlite", Path: ":memory:"}}).DataDir())
	assert.Empty(t, (&DB{config: &config.DatabaseConfig{Type: "sqlite"}}).DataDir())
	assert.Empty(t, (&DB{config: &config.DatabaseConfig{Type: "postgres", Path: filepath.Join(dir, "x")}}).DataDir(), "a remote database has no local data dir")
	assert.Empty(t, (*DB)(nil).DataDir())
}
