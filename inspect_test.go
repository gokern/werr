package werr_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gokern/werr"
)

func TestIsWrap(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		require.False(t, werr.IsWrap(nil))
	})

	t.Run("plain error", func(t *testing.T) {
		t.Parallel()
		require.False(t, werr.IsWrap(errors.New("plain")))
	})

	t.Run("werr error", func(t *testing.T) {
		t.Parallel()
		require.True(t, werr.IsWrap(werr.Wrap(errors.New("leaf"))))
	})

	t.Run("werr beneath fmt.Errorf", func(t *testing.T) {
		t.Parallel()
		require.True(t, werr.IsWrap(fmt.Errorf("ctx: %w", werr.Wrap(errors.New("leaf")))))
	})

	t.Run("errors.Join without werr", func(t *testing.T) {
		t.Parallel()

		joined := errors.Join(errors.New("a"), errors.New("b"))
		require.False(t, werr.IsWrap(joined))
	})

	t.Run("errors.Join containing werr", func(t *testing.T) {
		t.Parallel()
		// AsType[Error] traverses Unwrap() []error; werr inside a multi-error
		// must still be discoverable.
		joined := errors.Join(errors.New("plain"), werr.Wrap(errors.New("leaf")))
		require.True(t, werr.IsWrap(joined))
	})
}

func TestAsWrap(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		_, ok := werr.AsWrap(nil)
		require.False(t, ok)
	})

	t.Run("plain error", func(t *testing.T) {
		t.Parallel()

		_, ok := werr.AsWrap(errors.New("plain"))
		require.False(t, ok)
	})

	t.Run("werr error returns metadata", func(t *testing.T) {
		t.Parallel()

		err := werr.Wrapf(errors.New("leaf"), "loaded")

		w, ok := werr.AsWrap(err)
		require.True(t, ok)
		require.Equal(t, "loaded", w.Message())
		require.NotEmpty(t, w.FuncName())
	})

	t.Run("descends through fmt.Errorf via errors.AsType", func(t *testing.T) {
		t.Parallel()

		err := fmt.Errorf("ctx: %w", werr.Wrapf(errors.New("leaf"), "deep"))

		w, ok := werr.AsWrap(err)
		require.True(t, ok)
		require.Equal(t, "deep", w.Message())
	})
}

func TestStrip(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, werr.Strip(nil))
	})

	t.Run("non-werr returned unchanged", func(t *testing.T) {
		t.Parallel()

		plain := errors.New("plain")
		require.Same(t, plain, werr.Strip(plain))
	})

	t.Run("removes a single werr layer", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		require.Same(t, leaf, werr.Strip(werr.Wrap(leaf)))
	})

	t.Run("removes only one layer at a time", func(t *testing.T) {
		t.Parallel()

		err := werr.Wrap(werr.Wrap(errors.New("leaf")))
		stripped := werr.Strip(err)
		require.True(t, werr.IsWrap(stripped), "after one Strip the inner werr layer should remain")
	})

	t.Run("preserves errors.Is via the unwrapped error", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, werr.Strip(werr.Wrap(io.EOF)), io.EOF)
	})
}

func TestStripAll(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, werr.StripAll(nil))
	})

	t.Run("non-werr returned unchanged", func(t *testing.T) {
		t.Parallel()

		plain := errors.New("plain")
		require.Same(t, plain, werr.StripAll(plain))
	})

	t.Run("strips a werr-only chain to the leaf", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		err := werr.Wrap(werr.Wrap(werr.Wrap(leaf)))
		require.Same(t, leaf, werr.StripAll(err))
	})

	t.Run("stops at fmt.Errorf wrapper", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		fmtMid := fmt.Errorf("fmt: %w", leaf)
		err := werr.Wrap(werr.Wrap(fmtMid))

		require.Same(
			t,
			fmtMid,
			werr.StripAll(err),
			"StripAll must stop at the first non-werr wrapper",
		)
	})

	t.Run("result is never a werr.Error", func(t *testing.T) {
		t.Parallel()

		err := werr.Wrap(werr.Wrap(werr.Wrapf(errors.New("leaf"), "ctx")))
		require.False(t, werr.IsWrap(werr.StripAll(err)),
			"StripAll output must not be a werr.Error")
	})
}
