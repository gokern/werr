package werr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gokern/werr/v2"
)

func TestWalk(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		called := false
		root := werr.Walk(nil, func(werr.Frame) bool {
			called = true

			return true
		})

		require.NoError(t, root)
		require.False(t, called)
	})

	t.Run("non-werr error", func(t *testing.T) {
		t.Parallel()

		plain := errors.New("plain")
		called := false
		root := werr.Walk(plain, func(werr.Frame) bool {
			called = true

			return true
		})

		require.Same(t, plain, root)
		require.False(t, called, "fn must not be invoked for non-werr inputs")
	})

	t.Run("full chain", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		mid := fmt.Errorf("fmt: %w", leaf)
		l1 := werr.Wrapf(mid, "level 1")
		top := werr.Wrap(l1)

		var msgs []string

		root := werr.Walk(top, func(f werr.Frame) bool {
			msgs = append(msgs, f.Msg)
			require.NotEmpty(t, f.FuncName)
			require.NotEmpty(t, f.File)
			require.Positive(t, f.Line)

			return true
		})

		require.Equal(t, []string{"", "level 1"}, msgs, "frames are visited outermost to innermost")
		require.Same(t, mid, root, "root is the first non-werr error in the chain")
	})

	t.Run("early stop", func(t *testing.T) {
		t.Parallel()

		// Build a chain whose middle link's identity we can pin: outer wraps
		// `next`, `next` wraps the leaf. fn returns false on the outermost
		// frame, so Walk must return `next` itself (not the leaf, not the
		// outer error).
		next := werr.Wrap(errors.New("leaf"))
		err := werr.Wrapf(next, "outer")

		count := 0
		root := werr.Walk(err, func(werr.Frame) bool {
			count++

			return false
		})

		require.Equal(t, 1, count, "fn returning false must stop iteration after the first frame")
		require.Same(
			t,
			next,
			root,
			"early-stop must return the immediate inner werr.Error, not deeper or the outer link",
		)
	})

	t.Run("frame fields match Error accessors", func(t *testing.T) {
		t.Parallel()

		wrapped := werr.Wrapf(errors.New("leaf"), "boom")

		var got werr.Frame

		_ = werr.Walk(wrapped, func(f werr.Frame) bool {
			got = f

			return false
		})

		var w *werr.Error

		require.ErrorAs(t, wrapped, &w)
		require.Equal(t, w.File(), got.File)
		require.Equal(t, w.Line(), got.Line)
		require.Equal(t, w.FuncName(), got.FuncName)
		require.Equal(t, w.Message(), got.Msg)
	})

	t.Run("does not descend into errors.Join", func(t *testing.T) {
		t.Parallel()

		// Walk follows a single werr chain; errors.Join's Unwrap() []error
		// is not traversed.
		joined := errors.Join(werr.Wrap(errors.New("a")), werr.Wrap(errors.New("b")))

		called := false
		root := werr.Walk(joined, func(werr.Frame) bool {
			called = true

			return true
		})

		require.False(t, called, "Walk does not descend into multi-error joins")
		require.Same(t, joined, root)
	})

	t.Run("stops at fmt.Errorf wrapper mid-chain", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		inner := werr.Wrap(leaf)
		fmtMid := fmt.Errorf("ctx: %w", inner)
		top := werr.Wrap(fmtMid)

		count := 0
		root := werr.Walk(top, func(werr.Frame) bool {
			count++

			return true
		})

		require.Equal(t, 1, count, "Walk must stop at fmt.Errorf, not descend through it")
		require.Same(t, fmtMid, root)
	})

	t.Run("deep chain", func(t *testing.T) {
		t.Parallel()

		const depth = 32

		err := errors.New("leaf")
		for range depth {
			err = werr.Wrap(err)
		}

		count := 0

		_ = werr.Walk(err, func(werr.Frame) bool {
			count++

			return true
		})

		require.Equal(t, depth, count)
	})
}

// Walk must allocate nothing on its iteration path.
// testing.AllocsPerRun rejects parallel callers, so this is non-parallel.
func TestWalk_zeroAlloc(t *testing.T) {
	err := werr.Wrap(werr.Wrap(werr.Wrap(errors.New("leaf"))))

	var sink werr.Frame

	allocs := testing.AllocsPerRun(100, func() {
		_ = werr.Walk(err, func(f werr.Frame) bool {
			sink = f

			return true
		})
	})

	require.Zero(t, allocs, "Walk must not allocate; got %v allocs/op", allocs)

	_ = sink
}

func TestFrames(t *testing.T) {
	t.Parallel()

	t.Run("nil error yields nothing", func(t *testing.T) {
		t.Parallel()

		count := 0
		for range werr.Frames(nil) {
			count++
		}

		require.Zero(t, count)
	})

	t.Run("non-werr error yields nothing", func(t *testing.T) {
		t.Parallel()

		count := 0
		for range werr.Frames(errors.New("plain")) {
			count++
		}

		require.Zero(t, count)
	})

	t.Run("iterates outermost to innermost", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		l1 := werr.Wrapf(leaf, "level 1")
		l2 := werr.Wrapf(l1, "level 2")
		top := werr.Wrapf(l2, "top")

		var msgs []string
		for f := range werr.Frames(top) {
			msgs = append(msgs, f.Msg)
		}

		require.Equal(t, []string{"top", "level 2", "level 1"}, msgs)
	})

	t.Run("stops at first non-werr wrapper", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		fmtMid := fmt.Errorf("fmt: %w", leaf)
		l1 := werr.Wrap(fmtMid)
		top := werr.Wrap(l1)

		count := 0
		for range werr.Frames(top) {
			count++
		}

		require.Equal(t, 2, count, "Frames must stop at fmt.Errorf, not descend through it")
	})

	t.Run("early break works", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		top := werr.Wrapf(werr.Wrapf(werr.Wrapf(leaf, "a"), "b"), "c")

		var seen []string
		for f := range werr.Frames(top) {
			seen = append(seen, f.Msg)

			if f.Msg == "b" {
				break
			}
		}

		require.Equal(t, []string{"c", "b"}, seen)
	})
}

// Frames must allocate nothing on its iteration path.
// Non-parallel because testing.AllocsPerRun rejects parallel callers.
func TestFrames_zeroAlloc(t *testing.T) {
	leaf := errors.New("leaf")
	err := werr.Wrapf(werr.Wrapf(werr.Wrap(leaf), "ctx"), "top")

	var sink werr.Frame

	allocs := testing.AllocsPerRun(100, func() {
		for f := range werr.Frames(err) {
			sink = f
		}
	})

	require.Zero(t, allocs, "Frames must not allocate; got %v", allocs)

	_ = sink
}
