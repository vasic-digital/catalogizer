package logredact

import "runtime"

type runtimeMem struct{ total uint64 }

func (r *runtimeMem) read() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	r.total = m.TotalAlloc
}
