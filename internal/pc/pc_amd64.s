//go:build !werrsafe && amd64

#include "textflag.h"

// Caller returns the program counter of the caller of Caller's caller.
//
// With NOFRAME on amd64 the runtime inserts no prologue, so BP at entry
// still points at the caller's saved-BP slot per the Go internal ABI
// (https://go.dev/s/regabi). The caller's return address sits one word
// above that slot, so *(BP+8) is the program counter we want.
//
// runtime.getcallerpc uses the same idiom, so this is stable on the Go
// versions that publish a frame-pointer ABI on amd64 (Go 1.21+).
//
// func Caller() uintptr
TEXT ·Caller(SB),NOSPLIT|NOFRAME,$0-8
	MOVQ 8(BP), AX
	MOVQ AX, ret+0(FP)
	RET
