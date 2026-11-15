package pure

// QPerms is a plain in-tree concrete type with a pure method sharing its name with a DB-backed one.
type QPerms []string

func (p QPerms) HasQ(x string) bool { return len(p) > 0 && p[0] == x }
