//go:build !werrsafe && (amd64 || arm64)

package pc

// Caller returns the program counter of the caller of Caller's caller.
//
// Implemented in assembly on amd64 and arm64 — see pc_amd64.s and
// pc_arm64.s. The asm reads the return address one word above the frame
// pointer in a single MOV (~10ns, no allocations).
//
// The caller of pc.Caller must be marked //go:noinline so its stack frame
// exists at runtime; otherwise the asm reads the wrong frame.
func Caller() uintptr
