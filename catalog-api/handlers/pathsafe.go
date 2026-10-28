package handlers

// RED-phase stubs: permissive on purpose; replaced by the real implementation.

func containedJoin(root, rel string) (string, error) { return root + "/" + rel, nil }
func safeChildName(name string) (string, error)     { return name, nil }
func safeRemotePath(p string) error                  { return nil }
