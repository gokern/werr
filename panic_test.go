package werr_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gokern/werr"
)

func TestPanicToError(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, werr.PanicToError(nil))
	})

	t.Run("error value is wrapped and remains comparable via errors.Is", func(t *testing.T) {
		t.Parallel()

		input := errors.New("boom")
		got := werr.PanicToError(input)

		require.Error(t, got)
		require.ErrorIs(t, got, input)
		require.True(t, werr.IsWrap(got), "PanicToError must always return a werr.Error")
	})

	t.Run("string panic value is wrapped with the recovered text", func(t *testing.T) {
		t.Parallel()

		got := werr.PanicToError("string panic").Error()

		require.Contains(t, got, "panic recovered")
		require.Contains(t, got, "string panic")
	})

	t.Run("non-error non-string is rendered via %#v", func(t *testing.T) {
		t.Parallel()

		got := werr.PanicToError(123).Error()

		require.Contains(t, got, "panic recovered")
		require.Contains(t, got, "123")
	})

	t.Run("typed-nil error panic value does not crash the formatter", func(t *testing.T) {
		t.Parallel()

		// panic((*os.PathError)(nil)) produces a non-nil interface holding a
		// nil pointer. PanicToError's case error: branch must guard against
		// the typed nil — otherwise leaf.Error() inside the formatter
		// dereferences it and re-panics out of Error().
		var typedNil *typedNilError

		got := werr.PanicToError(typedNil)
		require.Error(t, got, "PanicToError must produce an error even for typed-nil panic values")

		require.NotPanics(t, func() {
			_ = got.Error()
		}, "rendering must not nil-deref the typed-nil leaf")
	})

	t.Run("typed-nil func error panic value does not crash the formatter", func(t *testing.T) {
		t.Parallel()

		// A nil func that satisfies error: calling Error() invokes the nil
		// func and panics. Mirrors the *Pointer guard for non-pointer nilable
		// kinds (Map, Chan, Func, Slice).
		var typedNil funcError

		got := werr.PanicToError(typedNil)
		require.Error(t, got)

		require.NotPanics(t, func() {
			_ = got.Error()
		}, "rendering must not invoke the nil func leaf")
	})

	t.Run("captures the panic site when used inside a deferred recover", func(t *testing.T) {
		t.Parallel()

		var got error

		func() {
			defer func() {
				if r := recover(); r != nil {
					got = werr.PanicToError(r)
				}
			}()

			_ = []string{}[1] // panic: index out of range
		}()

		require.Error(t, got)

		out := got.Error()
		require.Contains(t, out, "panic recovered")
		require.Contains(t, out, "runtime error: index out of range [1] with length 0")
		// Match runtime/panic.go loosely — gopanic may be renamed or inlined
		// across Go versions; we only care that the frame appears at all.
		require.Regexp(t, ` --- at runtime/panic\.go:\d+ \(\S+\)`, out)
		require.Regexp(t, ` --- at \S+/panic_test\.go:\d+ \(\S*TestPanicToError\S*\)`, out)
	})
}

func TestRecover(t *testing.T) {
	t.Parallel()

	t.Run("no panic leaves target untouched", func(t *testing.T) {
		t.Parallel()

		out := callWithRecover(func() { /* no panic */ })
		require.NoError(t, out)
	})

	t.Run("preserves a pre-existing error when no panic occurs", func(t *testing.T) {
		t.Parallel()

		preset := errors.New("preset")
		out := callWithRecoverPreset(preset, func() { /* no panic */ })
		require.Same(t, preset, out)
	})

	t.Run("captures error panic into target", func(t *testing.T) {
		t.Parallel()

		input := errors.New("boom")
		out := callWithRecover(func() { panic(input) })

		require.ErrorIs(t, out, input)
		require.True(t, werr.IsWrap(out))
	})

	t.Run("captures string panic into target", func(t *testing.T) {
		t.Parallel()

		out := callWithRecover(func() { panic("string panic") })

		require.Error(t, out)
		require.Contains(t, out.Error(), "string panic")
	})

	t.Run("captures runtime panic with stack frames", func(t *testing.T) {
		t.Parallel()

		out := callWithRecover(func() {
			_ = []string{}[1] // panic: index out of range
		})

		require.Error(t, out)
		s := out.Error()
		require.Contains(t, s, "runtime error: index out of range [1] with length 0")
		require.Regexp(t, ` --- at runtime/panic\.go:\d+ \(\S+\)`, s)
	})

	t.Run("nil target swallows the panic", func(t *testing.T) {
		t.Parallel()

		require.NotPanics(t, func() {
			func() {
				defer werr.Recover(nil)

				panic("ignored")
			}()
		})
	})

	t.Run("typed-nil error panic value renders without crashing", func(t *testing.T) {
		t.Parallel()

		var typedNil *typedNilError

		out := callWithRecover(func() { panic(typedNil) })
		require.Error(t, out)

		require.NotPanics(t, func() {
			_ = out.Error()
		}, "Recover must produce a renderable error for typed-nil panic values")
	})
}

// typedNilError is an error type used to trigger the "non-nil interface holding
// a nil pointer" panic shape: panic((*typedNilError)(nil)) recovers as a
// non-nil any whose dynamic type is *typedNilError and whose value is nil.
type typedNilError struct{ msg string }

func (e *typedNilError) Error() string { return e.msg }

// funcError is a non-pointer error implementation. A nil funcError satisfies the
// error interface but invoking Error() calls the nil func and panics — the
// scenario the panicValueToError guard must catch alongside the *Pointer
// case.
type funcError func() string

func (f funcError) Error() string { return f() }

// callWithRecover invokes fn, returning any panic-captured error via Recover.
//
//nolint:nonamedreturns // werr.Recover writes the wrapped panic into a named error return.
func callWithRecover(fn func()) (err error) {
	defer werr.Recover(&err)

	fn()

	return nil
}

// callWithRecoverPreset behaves like callWithRecover but starts with err set.
//
//nolint:nonamedreturns // werr.Recover writes the wrapped panic into a named error return.
func callWithRecoverPreset(initial error, fn func()) (err error) {
	err = initial
	defer werr.Recover(&err)

	fn()

	return err
}

// TestPanicToError_capturesCallerLine: the captured PC must resolve to
// the user's PanicToError call site, not to werr internals. Regression
// gate for the //go:noinline pragma on PanicToError; without it, the
// asm fast path reads the wrong frame and Line() drifts.
func TestPanicToError_capturesCallerLine(t *testing.T) {
	t.Parallel()

	var got error

	func() {
		defer func() {
			if r := recover(); r != nil {
				got = werr.PanicToError(r) // @panic-to-error
			}
		}()

		panic("boom")
	}()

	w, ok := werr.AsWrap(got)
	require.True(t, ok)
	require.Equal(t, "panic_test.go", filepathBase(w.File()))
	// No exact-line check — refactors here would break it. The function
	// name match is enough to confirm we resolved into this test.
	require.Contains(t, w.FuncName(), "TestPanicToError_capturesCallerLine")
}

// No equivalent test for Recover: it runs as the deferred function itself,
// and the "caller" of a deferred function is ambiguous between the asm
// path (user frame the runtime returns into) and runtime.Callers (the
// defer machinery). Either is defensible, but a strict-equality check
// would be brittle. PanicToError above covers the same regression without
// that ambiguity.

func filepathBase(str string) string {
	for i := len(str) - 1; i >= 0; i-- {
		if str[i] == '/' || str[i] == '\\' {
			return str[i+1:]
		}
	}

	return str
}
