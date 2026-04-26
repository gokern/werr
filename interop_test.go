package werr_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gokern/werr"
)

// customError is a representative third-party error type used to exercise
// the boundaries between werr and user-defined errors. It implements:
//
//   - Error() string — required by the interface.
//   - Unwrap() error — so errors.Is/errors.As traverse through it.
//   - Is(target error) bool — to match peers by Code (the common
//     "category-comparable" idiom seen in pgconn.PgError, gRPC status,
//     etc.).
type customError struct {
	Code int
	err  error
}

func (e *customError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("custom[%d]", e.Code)
	}

	return fmt.Sprintf("custom[%d]: %v", e.Code, e.err)
}

func (e *customError) Unwrap() error { return e.err }

func (e *customError) Is(target error) bool {
	other, ok := target.(*customError)

	return ok && other.Code == e.Code
}

// --- Group A: werr wraps a custom error ---

func TestInterop_werrWrapsCustom(t *testing.T) {
	t.Parallel()

	cust := &customError{Code: 42, err: io.EOF}
	wrapped := werr.Wrap(cust)

	t.Run("IsWrap finds the werr layer", func(t *testing.T) {
		t.Parallel()
		require.True(t, werr.IsWrap(wrapped))
	})

	t.Run("AsWrap returns the werr layer", func(t *testing.T) {
		t.Parallel()

		w, ok := werr.AsWrap(wrapped)
		require.True(t, ok)
		require.NotEmpty(t, w.FuncName())
	})

	t.Run("Strip returns the custom error directly", func(t *testing.T) {
		t.Parallel()

		stripped := werr.Strip(wrapped)
		require.Same(t, cust, stripped)
	})

	t.Run("StripAll stops at the custom error (not werr)", func(t *testing.T) {
		t.Parallel()

		nested := werr.Wrap(werr.Wrap(cust))
		require.Same(t, cust, werr.StripAll(nested))
	})

	t.Run("Walk visits the werr frame and returns the custom error as leaf", func(t *testing.T) {
		t.Parallel()

		count := 0
		root := werr.Walk(wrapped, func(werr.Frame) bool {
			count++

			return true
		})

		require.Equal(t, 1, count)
		require.Same(t, cust, root, "Walk's leaf is the first non-werr error in the chain")
	})

	t.Run("errors.Is finds the inner sentinel through werr and custom", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, wrapped, io.EOF)
	})

	t.Run("errors.Is matches custom.Is via Code through werr", func(t *testing.T) {
		t.Parallel()
		// customError.Is matches by Code; the comparand needs the same Code,
		// not the same instance.
		require.ErrorIs(t, wrapped, &customError{Code: 42})
		require.NotErrorIs(t, wrapped, &customError{Code: 99})
	})

	t.Run("errors.As extracts *customError through the werr layer", func(t *testing.T) {
		t.Parallel()

		var got *customError

		require.ErrorAs(t, wrapped, &got)
		require.Equal(t, 42, got.Code)
		require.Same(t, cust, got)
	})
}

// --- Group B: custom error contains werr inside ---

func TestInterop_customWrapsWerr(t *testing.T) {
	t.Parallel()

	leaf := errors.New("leaf")
	inner := werr.Wrapf(leaf, "ctx")
	cust := &customError{Code: 7, err: inner}

	t.Run("IsWrap traverses through custom.Unwrap", func(t *testing.T) {
		t.Parallel()
		require.True(t, werr.IsWrap(cust))
	})

	t.Run("AsWrap returns the inner werr layer", func(t *testing.T) {
		t.Parallel()

		w, ok := werr.AsWrap(cust)
		require.True(t, ok)
		require.Equal(t, "ctx", w.Message())
	})

	t.Run("Strip leaves the custom error untouched", func(t *testing.T) {
		t.Parallel()
		// cust is NOT a werr.Error, so Strip returns it unchanged.
		require.Same(t, cust, werr.Strip(cust))
	})

	t.Run("StripAll leaves the custom error untouched", func(t *testing.T) {
		t.Parallel()
		require.Same(t, cust, werr.StripAll(cust))
	})

	t.Run("Walk does not descend into custom.Unwrap", func(t *testing.T) {
		t.Parallel()

		called := false
		root := werr.Walk(cust, func(werr.Frame) bool {
			called = true

			return true
		})

		require.False(t, called, "Walk stops at non-werr errors regardless of their Unwrap shape")
		require.Same(t, cust, root)
	})

	t.Run("errors.Is finds leaf through custom.Unwrap and werr", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, cust, leaf)
	})

	t.Run("errors.As finds *werr.Error through custom.Unwrap", func(t *testing.T) {
		t.Parallel()

		var w *werr.Error

		require.ErrorAs(t, cust, &w)
		require.Equal(t, "ctx", w.Message())
	})
}

// --- Group C: errors.Join interop ---

func TestInterop_errorsJoin(t *testing.T) {
	t.Parallel()

	t.Run("AsWrap finds werr nested inside one Join branch via fmt.Errorf", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		hidden := fmt.Errorf("ctx: %w", werr.Wrapf(leaf, "deep"))
		joined := errors.Join(errors.New("plain"), hidden)

		w, ok := werr.AsWrap(joined)
		require.True(t, ok)
		require.Equal(t, "deep", w.Message())
	})

	t.Run("IsWrap finds werr in any branch of a deep Join structure", func(t *testing.T) {
		t.Parallel()

		// Two-level Join: outer joins (Join(plain, plain), Join(plain, werr)).
		buried := errors.Join(errors.New("a"), werr.Wrap(errors.New("leaf")))
		outer := errors.Join(errors.Join(errors.New("b"), errors.New("c")), buried)

		require.True(t, werr.IsWrap(outer))
	})

	t.Run("errors.Is through werr that wraps a Join containing the sentinel", func(t *testing.T) {
		t.Parallel()

		joined := errors.Join(errors.New("first"), io.EOF)
		wrapped := werr.Wrapf(joined, "ctx")

		require.ErrorIs(t, wrapped, io.EOF)
	})

	t.Run("Walk stops at Join because Join is not a werr.Error", func(t *testing.T) {
		t.Parallel()

		joined := errors.Join(werr.Wrap(errors.New("a")), werr.Wrap(errors.New("b")))
		wrapped := werr.Wrap(joined)

		count := 0
		root := werr.Walk(wrapped, func(werr.Frame) bool {
			count++

			return true
		})

		require.Equal(t, 1, count, "Walk visits the outer werr layer, then stops at Join")
		require.Same(t, joined, root)
	})
}

// --- Group D: mixed multi-tier chain ---

// werr → customError → fmt.Errorf → werr → leaf:
// every layer is a different wrapping mechanism. errors.Is/As must
// traverse all of them; werr-specific helpers must respect the boundary.
func TestInterop_mixedChain(t *testing.T) {
	t.Parallel()

	leaf := errors.New("leaf")
	innerWerr := werr.Wrapf(leaf, "inner")
	fmtMid := fmt.Errorf("fmt: %w", innerWerr)
	cust := &customError{Code: 1, err: fmtMid}
	top := werr.Wrapf(cust, "top")

	t.Run("AsWrap finds the outermost werr (top)", func(t *testing.T) {
		t.Parallel()

		w, ok := werr.AsWrap(top)
		require.True(t, ok)
		require.Equal(t, "top", w.Message())
	})

	t.Run("Strip exposes the custom error", func(t *testing.T) {
		t.Parallel()
		require.Same(t, cust, werr.Strip(top))
	})

	t.Run("StripAll stops at the custom error (one werr layer at top)", func(t *testing.T) {
		t.Parallel()
		require.Same(t, cust, werr.StripAll(top))
	})

	t.Run("Walk visits the top werr frame, then stops at the custom error", func(t *testing.T) {
		t.Parallel()

		var msgs []string

		root := werr.Walk(top, func(f werr.Frame) bool {
			msgs = append(msgs, f.Msg)

			return true
		})

		require.Equal(t, []string{"top"}, msgs,
			"Walk does not descend through customError, even though it has Unwrap")
		require.Same(t, cust, root)
	})

	t.Run("errors.Is finds the leaf through every wrapper", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, top, leaf)
	})

	t.Run("errors.As extracts *customError through the top werr", func(t *testing.T) {
		t.Parallel()

		var got *customError

		require.ErrorAs(t, top, &got)
		require.Equal(t, 1, got.Code)
	})

	t.Run("errors.As finds the inner werr.Error", func(t *testing.T) {
		t.Parallel()
		// Top werr is the outermost; AsType[*Error] returns the first match,
		// so this just confirms "any *werr.Error at all is reachable".
		var w *werr.Error
		require.ErrorAs(t, top, &w)
		require.NotEmpty(t, w.Message())
	})
}
