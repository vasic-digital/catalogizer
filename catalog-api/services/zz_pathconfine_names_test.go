package services

import "catalogizer/internal/pathguard"

// env names, via the package constants so a rename cannot silently desync the tests
const (
	pathGuardEnvSync = pathguard.EnvSyncRoots
	pathGuardEnvRead = pathguard.EnvReadRoots
	pathGuardEnvConv = pathguard.EnvConversionRoots
)
