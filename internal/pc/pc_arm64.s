//go:build !werrsafe && arm64

#include "textflag.h"

// Caller returns the program counter of the caller of Caller's caller.
//
// With NOFRAME on arm64 the runtime inserts no prologue, so R29 (the
// frame-pointer register per the Go internal ABI, https://go.dev/s/regabi)
// at entry still points at the caller's saved-FP slot. The caller's return
// address sits one word above that slot, so *(R29+8) is the program
// counter we want.
//
// runtime.getcallerpc uses the same idiom, so this is stable on the Go
// versions that publish a frame-pointer ABI on arm64 (Go 1.21+).
//
// func Caller() uintptr
TEXT ·Caller(SB),NOSPLIT|NOFRAME,$0-8
	MOVD 8(R29), R20
	MOVD R20, ret+0(FP)
	RET
