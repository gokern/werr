package werr_test

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/gokern/panics"
	"github.com/stretchr/testify/require"

	"github.com/gokern/werr/v2"
)

func TestCallers_nilErr_returnsNil(t *testing.T) {
	t.Parallel()

	require.Nil(t, werr.Callers(nil))
}

func TestCallers_nonWerr_returnsNil(t *testing.T) {
	t.Parallel()

	t.Run("plain stdlib error", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, werr.Callers(errors.New("plain")))
	})

	t.Run("io.EOF sentinel", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, werr.Callers(io.EOF))
	})
}

// makeInner / makeMiddle / makeOuter give us three distinct call-site PCs
// so the order tests below can identify each frame by function name.
//
//go:noinline so each wrap has a real stack frame for the asm pc.Caller path.
func makeInner(leaf error) error { return werr.Wrap(leaf) }

//go:noinline so each wrap has a real stack frame for the asm pc.Caller path.
func makeMiddle(leaf error) error { return werr.Wrapf(makeInner(leaf), "middle") }

//go:noinline so each wrap has a real stack frame for the asm pc.Caller path.
func makeOuter(leaf error) error { return werr.Wrapf(makeMiddle(leaf), "outer") }

func TestCallers_orderInnermostFirst(t *testing.T) {
	t.Parallel()

	pcs := werr.Callers(makeOuter(io.EOF))
	require.Len(t, pcs, 3)

	names := make([]string, len(pcs))
	for i, pc := range pcs {
		fn := runtime.FuncForPC(pc)
		require.NotNil(t, fn, "FuncForPC returned nil for index %d", i)
		names[i] = fn.Name()
	}

	require.Contains(t, names[0], "makeInner",
		"index 0 must be the innermost wrap (closest to leaf)")
	require.Contains(t, names[1], "makeMiddle",
		"index 1 must be the middle wrap")
	require.Contains(t, names[2], "makeOuter",
		"index 2 must be the outermost wrap (furthest from leaf)")
}

func TestCallers_stopsAtFmtErrorfBoundary(t *testing.T) {
	t.Parallel()

	// Chain: werr.Wrapf -> fmt.Errorf -> werr.Wrap -> io.EOF. Callers must
	// stop at the fmt.Errorf link and return only the outer werr-frame.
	inner := werr.Wrap(io.EOF)
	mid := fmt.Errorf("crossing %w", inner)
	outer := werr.Wrapf(mid, "outer")

	pcs := werr.Callers(outer)
	require.Len(t, pcs, 1, "must contain only the outer werr-frame")

	fn := runtime.FuncForPC(pcs[0])
	require.NotNil(t, fn)
	require.Contains(t, fn.Name(), "TestCallers_stopsAtFmtErrorfBoundary",
		"the single PC must come from the outer wrap site, which lives in this test")
}

func TestCallers_outerNotWerr_returnsNil(t *testing.T) {
	t.Parallel()

	// Outermost link is *fmt.wrapError, not *werr.Error. Callers checks
	// the head of the chain and bails immediately, same as Walk.
	wrapped := fmt.Errorf("crossing %w", werr.Wrap(io.EOF))
	require.Nil(t, werr.Callers(wrapped))
}

func TestCallers_errorsJoinReturnsNil(t *testing.T) {
	t.Parallel()

	// errors.Join produces *errors.joinError, which isn't a *werr.Error.
	// Callers bails at the head, same as for fmt.Errorf, even though both
	// joined branches are werr frames. There's no single linear stack for
	// a tree of errors.
	joined := errors.Join(werr.Wrap(io.EOF), werr.Wrap(errors.New("other")))
	require.Nil(t, werr.Callers(joined))
}

func TestCallers_typedNilStackTraceReturnsNil(t *testing.T) {
	t.Parallel()

	// sentry-go calls StackTrace() through reflection on any value that
	// has the method, including a typed-nil *werr.Error (e.g. `var e
	// *werr.Error` handed back as an `error`). It must not panic; the
	// typed-nil guard inside Callers handles it.
	var e *werr.Error

	require.NotPanics(t, func() {
		require.Nil(t, e.StackTrace())
	})
}

func TestCallers_deepChain(t *testing.T) {
	t.Parallel()

	// Chain depth past 16, bigger than the inline buffers in Error.Error,
	// Pretty, OneLine. Callers itself doesn't have an inline buffer
	// (two-pass count + single make), but if anyone ever adds one, this
	// test is the trip wire.
	const depth = 32

	err := io.EOF
	for range depth {
		err = werr.Wrap(err)
	}

	pcs := werr.Callers(err)
	require.Len(t, pcs, depth, "must collect every werr-frame")

	for i, pc := range pcs {
		fn := runtime.FuncForPC(pc)
		require.NotNil(t, fn, "FuncForPC returned nil for pcs[%d]", i)
		require.NotEmpty(t, fn.Name(),
			"every PC must resolve to a function name regardless of chain depth")
	}
}

func TestCallers_singleAllocation(t *testing.T) {
	// AllocsPerRun panics under t.Parallel(), so this test is sequential.
	// Build the chain once outside the measured callback.
	err := io.EOF
	for range 5 {
		err = werr.Wrap(err)
	}

	allocs := testing.AllocsPerRun(100, func() {
		_ = werr.Callers(err)
	})

	require.LessOrEqual(t, allocs, 1.0,
		"Callers must allocate exactly the result slice and nothing else; got %.2f", allocs)
}

//go:noinline
func raisePanic() { panic("boom") }

// A recovered panic carries a complete goroutine stack, down to
// runtime.goexit. The wrap sites are frames from that same stack, so
// appending them would place code outside goexit — a trace that cannot
// exist. Callers therefore reports wrap sites only, and the panic stack
// stays reachable, unspliced, through panics.As.
func TestCallers_excludesThePanicStack(t *testing.T) {
	t.Parallel()

	err := werr.Wrapf(panics.Catch(raisePanic), "delivering")

	pcs := werr.Callers(err)
	require.Len(t, pcs, 1, "only the wrap site belongs to werr")

	fn := runtime.FuncForPC(pcs[0])
	require.NotNil(t, fn)
	require.Contains(t, fn.Name(), "TestCallers_excludesThePanicStack",
		"the one PC must be this test's Wrapf call site")

	p, ok := panics.As(err)
	require.True(t, ok, "the panic must still be reachable")

	frame, _ := runtime.CallersFrames(p.StackTrace()).Next()
	require.True(t, strings.HasSuffix(frame.Function, ".raisePanic"),
		"panics.As is where the panic site lives, got %q", frame.Function)
}

func TestCallers_bareErrorStillReturnsNil(t *testing.T) {
	t.Parallel()

	// A *panics.Panic that is not wrapped in a *werr.Error is not werr's to
	// report; sentry finds its StackTrace() directly.
	require.Nil(t, werr.Callers(panics.Catch(raisePanic)))
}
