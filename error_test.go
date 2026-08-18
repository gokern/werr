package werr

import (
	"errors"
	"fmt"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestError_Accessors(t *testing.T) {
	t.Parallel()

	t.Run("populated by Wrapf", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		err := Wrapf(leaf, "ctx")

		we := &Error{}
		ok := errors.As(err, &we)
		require.True(t, ok)

		require.Equal(t, "ctx", we.Message())
		require.NotEmpty(t, we.FuncName())
		require.NotEmpty(t, we.File())
		require.Positive(t, we.Line())
		require.NotZero(t, we.PC())
		require.Same(t, leaf, we.Unwrap())
	})

	t.Run("zero pc renders empty values without panicking", func(t *testing.T) {
		t.Parallel()

		// runtime.Callers can theoretically return 0 for very shallow stacks
		// or after future Go runtime changes. The accessors must remain safe.
		e := &Error{err: errors.New("leaf")}

		require.Empty(t, e.FuncName())
		require.Empty(t, e.File())
		require.Zero(t, e.Line())
		require.Equal(t, uintptr(0), e.PC())
	})
}

// errors.Unwrap follows the chain frame by frame.
func TestErrorsUnwrap_throughChain(t *testing.T) {
	t.Parallel()

	leaf := errors.New("leaf")
	mid := Wrapf(leaf, "mid")
	top := Wrapf(mid, "top")

	// werr.Error is a pointer drawn from an arena pool, so errors.Unwrap
	// returns the wrapped pointer instance directly. Unwrap(top) is the
	// same *Error as mid.
	require.Same(t, mid, errors.Unwrap(top))
	require.Same(t, leaf, errors.Unwrap(mid)) // leaf is the inner err pointer
	require.NoError(t, errors.Unwrap(leaf))
}

// errors.Is finds sentinels anywhere in a mixed werr / fmt.Errorf chain.
func TestErrorsIs_findsTargetInChain(t *testing.T) {
	t.Parallel()

	leaf := errors.New("leaf")
	fmtWrap := fmt.Errorf("ctx: %w", leaf)
	mid := Wrapf(fmtWrap, "mid")
	top := Wrapf(mid, "top")

	t.Run("matches leaf sentinel", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, top, leaf)
	})

	t.Run("matches fmt-wrapped intermediate", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, top, fmtWrap)
	})

	t.Run("does not match unrelated error", func(t *testing.T) {
		t.Parallel()
		require.NotErrorIs(t, top, errors.New("unrelated"))
	})
}

// errors.As extracts the werr.Error type from any depth of the chain.
func TestErrorsAs_extractsErrorType(t *testing.T) {
	t.Parallel()

	leaf := errors.New("leaf")
	mid := Wrapf(leaf, "mid")
	top := Wrapf(mid, "top")

	t.Run("from outermost werr", func(t *testing.T) {
		t.Parallel()

		var w *Error

		require.ErrorAs(t, top, &w)
		require.Equal(t, "top", w.Message())
	})

	t.Run("from intermediate werr", func(t *testing.T) {
		t.Parallel()

		var w *Error

		require.ErrorAs(t, mid, &w)
		require.Equal(t, "mid", w.Message())
	})

	t.Run("absent for plain errors", func(t *testing.T) {
		t.Parallel()

		var w *Error

		require.NotErrorAs(t, errors.New("plain"), &w)
	})
}

func TestError_structSizeIsPinned(t *testing.T) {
	t.Parallel()

	// pc(1 word) + err(2 words) + msg(2 words) = 5 words, and 1024 of them
	// per arena slab: 40 bytes on a 64-bit target, 20 on a 32-bit one, which
	// internal/pc's runtime.Callers fallback keeps supported. Expressed in
	// words rather than pinned at 40 so the assertion holds on both.
	//
	// A panic stack must never become a field here: it would cost every
	// error, and 99.99% of them are not panics. panics.Panic lives behind
	// the err field instead.
	const words = 5

	require.Equal(t, unsafe.Sizeof(uintptr(0))*words, unsafe.Sizeof(Error{}),
		"werr.Error must stay %d words wide", words)
}
