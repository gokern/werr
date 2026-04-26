// Package pc captures a single program counter for the caller of the
// caller of [Caller]. Used by werr to attribute a wrap to its call site
// at ~10ns, vs ~150ns for runtime.Callers.
//
// # Build paths
//
//   - amd64/arm64 (default): asm helper reads the return address one word
//     above the frame pointer.
//   - Any other arch, or werrsafe build tag: runtime.Callers fallback.
//
// # Frame-pointer ABI
//
// The asm helpers depend on Go's frame-pointer convention, part of the Go
// internal ABI since Go 1.21:
//
//   - https://go.dev/s/regabi
//   - https://pkg.go.dev/cmd/internal/obj/arm64
//
// runtime.getcallerpc inside the Go runtime uses the same idiom.
//
// # Adding more architectures
//
// ppc64le, riscv64, and s390x are straightforward (~30 LOC + frame-pointer
// ABI verification each) but deferred until requested. Open an issue with
// your platform and a benchmark target.
package pc
