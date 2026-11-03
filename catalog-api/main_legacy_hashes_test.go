package main

import (
	"strings"
	"testing"

	"catalogizer/internal/metrics"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real sqlite DB (no mocks): the sunset counter must count only rows still in
// the legacy hex SHA-256 shape, never v1 bcrypt or v2 "$cz1$" rows.
func TestReportLegacyPasswordHashes_CountsOnlyLegacyRows(t *testing.T) {
	db := seedTestDB(t)
	addLegacyAdmin(t, db, "legacy1", "Str0ng!Operator-Value-91")
	addLegacyAdmin(t, db, "legacy2", "Str0ng!Operator-Value-92")
	require.NoError(t, seedDefaultAdmin(db, "v2admin", "Str0ng!Operator-Value-93")) // v2 row
	_, err := db.Exec(`INSERT INTO users (username,email,password_hash,salt,role_id,first_name,last_name,display_name,is_active) VALUES ('v1user','v1@x','$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ0123','s',2,'a','b','c',1)`)
	require.NoError(t, err)

	var lines []string
	n, err := reportLegacyPasswordHashes(db, func(f string, a ...any) { lines = append(lines, f) })
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, 2.0, gaugeValue(t))
	require.Len(t, lines, 1, "a non-zero count must be logged once")
	assert.True(t, strings.Contains(lines[0], "legacy"), lines[0])
}

func TestReportLegacyPasswordHashes_ZeroIsQuietAndGaugeZero(t *testing.T) {
	db := seedTestDB(t)
	require.NoError(t, seedDefaultAdmin(db, "v2admin", "Str0ng!Operator-Value-93"))
	called := false
	n, err := reportLegacyPasswordHashes(db, func(string, ...any) { called = true })
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, called)
	assert.Equal(t, 0.0, gaugeValue(t))
}

func gaugeValue(t *testing.T) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, metrics.LegacyPasswordHashes.Write(&m))
	return m.GetGauge().GetValue()
}
