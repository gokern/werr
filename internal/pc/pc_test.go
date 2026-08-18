package pc_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gokern/werr/v2/internal/pc"
)

// captureFromHelper routes the call through one //go:noinline frame.
// pc.Caller's contract is "return the PC of the caller of Caller's
// caller", so the resolved PC must point at the test, not the helper.
//
//go:noinline
func captureFromHelper() uintptr {
	return pc.Caller()
}

func TestCaller_returnsCallerOfCallersCaller(t *testing.T) {
	t.Parallel()

	_, _, expectedLine, _ := runtime.Caller(0)
	got := captureFromHelper() // expected line: expectedLine + 1

	require.NotZero(t, got, "Caller must return a non-zero PC")

	rfn := runtime.FuncForPC(got)
	require.NotNil(t, rfn, "FuncForPC must resolve the captured PC")

	_, gotLine := rfn.FileLine(got)
	require.Equal(t, expectedLine+1, gotLine,
		"captured PC must resolve to the line of the helper call in the test")
}

// captureBoth returns (asmPC, runtimeCallersPC) from one call site. On
// the werrsafe build both come from runtime.Callers; on the default
// build the first comes from the asm helper.
//
//go:noinline
func captureBoth() (uintptr, uintptr) {
	asm := pc.Caller()

	var pcs [1]uintptr

	n := runtime.Callers(2, pcs[:])
	if n == 0 {
		return asm, 0
	}

	return asm, pcs[0]
}

// pc.Caller and runtime.Callers must resolve to the same file and line
// when called from the same site — the contract werr relies on when
// swapping runtime.Callers for the asm helper.
func TestCaller_equivalentToRuntimeCallers(t *testing.T) {
	t.Parallel()

	asmPC, callersPC := captureBoth()
	require.NotZero(t, asmPC)
	require.NotZero(t, callersPC)

	asmFn := runtime.FuncForPC(asmPC)
	callersFn := runtime.FuncForPC(callersPC)

	require.NotNil(t, asmFn)
	require.NotNil(t, callersFn)
	require.Equal(t, asmFn.Name(), callersFn.Name(),
		"asm and runtime.Callers must report the same function")

	asmFile, asmLine := asmFn.FileLine(asmPC)
	callersFile, callersLine := callersFn.FileLine(callersPC)
	require.Equal(t, asmFile, callersFile)
	require.Equal(t, asmLine, callersLine,
		"asm and runtime.Callers must resolve to the same line")
}

// pc.Caller must allocate nothing — every werr.Wrap goes through it.
// Non-parallel because testing.AllocsPerRun rejects parallel callers.
func TestCaller_zeroAlloc(t *testing.T) {
	var sink uintptr

	allocs := testing.AllocsPerRun(100, func() {
		sink = captureFromHelper()
	})

	require.Zero(t, allocs, "pc.Caller must not allocate; got %v", allocs)
	require.NotZero(t, sink, "sanity: captured PC should be non-zero")
}
