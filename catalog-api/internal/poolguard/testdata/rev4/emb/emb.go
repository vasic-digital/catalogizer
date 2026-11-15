package emb

import (
	"context"

	"catalogizer/internal/poolguard/testdata/rev4/ifc"
)

type Inner struct{ ifc.T }
type W struct{ *Inner }
type N struct{ X ifc.T }

var _ = context.Background
