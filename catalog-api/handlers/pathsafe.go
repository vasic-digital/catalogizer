package handlers

import "catalogizer/internal/pathsafe"

// Thin delegates: the single implementation lives in internal/pathsafe so the
// production handlers (internal/handlers) and this package cannot drift.

var errPathEscapes = pathsafe.ErrPathEscapes

func safeChildName(name string) (string, error) { return pathsafe.SafeChildName(name) }

func safeRemotePath(p string) error { return pathsafe.SafeRemotePath(p) }

func containedJoin(root, rel string) (string, error) { return pathsafe.ContainedJoin(root, rel) }
