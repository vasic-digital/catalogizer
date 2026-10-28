package main

import (
	"fmt"
	"os"

	"catalogizer/config"
	"catalogizer/database"
)

// runDBMaintenance implements the offline `db-rekey` and `db-restore`
// subcommands (CAT-001). It prints paths only - never key material - and
// returns the process exit code.
func runDBMaintenance(cfg *config.DatabaseConfig, args []string) int {
	if cfg.Type != "" && cfg.Type != "sqlite" {
		fmt.Fprintln(os.Stderr, "db maintenance commands apply to SQLite only (database.type must be sqlite)")
		return 2
	}
	switch args[0] {
	case "db-rekey":
		res, err := database.Rekey(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rekey FAILED: %v\n", err)
			return 1
		}
		fmt.Printf("rekey OK\n  active key file : %s\n  pre-rekey backup: %s (encrypted with the OLD key)\n  old key backup  : %s\n", res.KeyFile, res.BackupPath, res.OldKeyBackupPath)
		fmt.Println("Keep the old key backup as long as you keep the pre-rekey backup; delete both securely when no longer needed.")
		return 0
	case "db-restore":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: catalog-api db-restore <backup-file>")
			return 2
		}
		if err := database.RestoreBackup(cfg, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "restore FAILED: %v\n", err)
			return 1
		}
		fmt.Printf("restore OK: %s -> %s (previous database kept as %s.pre-restore-*)\n", args[1], cfg.Path, cfg.Path)
		return 0
	}
	return 2
}
