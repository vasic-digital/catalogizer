package p

// Stale nested copy: the build uses the replace target, not this directory.
type T struct{}

func (T) Load() error { return nil }
