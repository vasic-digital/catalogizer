package good

import "testing"

func TestAdd(t *testing.T)  { if 2+3 != 5 { t.Fatal("add") } }
func TestSub(t *testing.T)  { if 5-3 != 2 { t.Fatal("sub") } }
func TestSkipped(t *testing.T) { t.Skip("seeded skip") }
