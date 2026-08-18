//go:build werrsafe || !(amd64 || arm64)

package pc

import "runtime"

// Caller returns the program counter of the caller of Caller's caller.
//
// Portable fallback delegating to runtime.Callers. The asm fast path
// (pc_unsafe.go + pc_<arch>.s) is used on amd64 and arm64 when the
// werrsafe build tag is not set.
//
//go:noinline keeps the skip count below correct: if Caller were inlined
// into werr.Wrap, runtime.Callers(3, …) would skip one frame too many
// and capture the user's caller instead of the user. The asm path is
// structurally non-inlinable, so this matters only on the safe path.
func Caller() uintptr {
	// Skip 3: runtime.Callers, pc.Caller, pc.Caller's caller (e.g. werr.Wrap).
	// Frame 3 is the user code we want to attribute the wrap to.
	//
	// The count holds only because every caller is a wrap entry point invoked
	// directly by user code. Call pc.Caller from a deferred function and
	// frame 3 lands on the defer-dispatch site rather than the user's source
	// line. The call-site tests in werr's wrap_test.go are the gate for both
	// this path and the asm one.
	var pcs [1]uintptr
	if n := runtime.Callers(3, pcs[:]); n == 0 {
		return 0
	}
	return pcs[0]
}
