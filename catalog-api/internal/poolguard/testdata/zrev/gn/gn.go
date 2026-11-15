package gn

type Box[T any] struct{ v T }

func (b Box[T]) Load() error { return nil }

type Loader interface{ Load() error }
type Call[T Loader] struct{ v T }

func (c Call[T]) Load() error { return c.v.Load() }
