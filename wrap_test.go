package werr_test

import (
	_ "embed"
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gokern/werr"
)

//go:embed wrap_test.go
var wrapTestSource string

func TestWrap(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, werr.Wrap(nil))
	})

	t.Run("wraps an error preserving identity", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		got := werr.Wrap(leaf)

		var w *werr.Error
		require.ErrorAs(t, got, &w)
		require.ErrorIs(t, got, leaf)
	})
}

func TestWrapf(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, werr.Wrapf(nil, "ignored"))
	})

	t.Run("formats and stores the message", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		got := werr.Wrapf(leaf, "context: %s=%d", "key", 7)

		require.ErrorIs(t, got, leaf)

		var w *werr.Error

		require.ErrorAs(t, got, &w)
		require.Equal(t, "context: key=7", w.Message())
	})

	t.Run("preserves multibyte characters in the message", func(t *testing.T) {
		t.Parallel()

		got := werr.Wrapf(errors.New("leaf"), "файл не найден: %s", "/опт/конфиг")

		var w *werr.Error

		require.ErrorAs(t, got, &w)
		require.Equal(t, "файл не найден: /опт/конфиг", w.Message())
	})
}

func TestWrap3(t *testing.T) {
	t.Parallel()

	pass := func(err error) (int, string, error) { return 100, "ctx", err }

	t.Run("forwards values when err is nil", func(t *testing.T) {
		t.Parallel()

		a, b, err := werr.Wrap3(pass(nil))
		require.NoError(t, err)
		require.Equal(t, 100, a)
		require.Equal(t, "ctx", b)
	})

	t.Run("wraps non-nil err and forwards both values", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		a, b, err := werr.Wrap3(pass(leaf))

		require.Equal(t, 100, a)
		require.Equal(t, "ctx", b)

		var w *werr.Error
		require.ErrorAs(t, err, &w)
		require.ErrorIs(t, err, leaf)
	})
}

func TestWrap2(t *testing.T) {
	t.Parallel()

	pass := func(err error) (int, error) { return 100, err }

	t.Run("forwards value when err is nil", func(t *testing.T) {
		t.Parallel()

		v, err := werr.Wrap2(pass(nil))
		require.NoError(t, err)
		require.Equal(t, 100, v)
	})

	t.Run("wraps non-nil err and forwards value", func(t *testing.T) {
		t.Parallel()

		leaf := errors.New("leaf")
		v, err := werr.Wrap2(pass(leaf))

		require.Equal(t, 100, v)

		var w *werr.Error
		require.ErrorAs(t, err, &w)
		require.ErrorIs(t, err, leaf)
	})

	t.Run("preserves the value type for non-error generic", func(t *testing.T) {
		t.Parallel()

		fn := func() (struct{ name string }, error) {
			return struct{ name string }{name: "alice"}, errors.New("leaf")
		}

		v, err := werr.Wrap2(fn())
		require.Equal(t, "alice", v.name)
		require.True(t, werr.IsWrap(err))
	})

	t.Run("forwards typed-nil pointer when err is non-nil", func(t *testing.T) {
		t.Parallel()

		type box struct{}

		fn := func() (*box, error) { return nil, errors.New("leaf") }
		v, err := werr.Wrap2(fn())

		require.Nil(t, v, "typed-nil value must be forwarded as-is")
		require.True(t, werr.IsWrap(err))
	})
}

// Wrap3 must forward typed-nil/zero values verbatim alongside a non-nil
// err — covers the case the original TestWrap3 does not exercise (it
// forwards 100 / "ctx", not zero values).
func TestWrap3_typedNilForwarded(t *testing.T) {
	t.Parallel()

	type box struct{}

	fn := func() (*box, int, error) { return nil, 0, errors.New("leaf") }
	a, b, err := werr.Wrap3(fn())

	require.Nil(t, a)
	require.Zero(t, b)
	require.True(t, werr.IsWrap(err))
}

// Hammer the wrap path under -race so a future regression (e.g. a stray
// package-level cache) shows up as a data race.
func TestWrap_concurrent(t *testing.T) {
	t.Parallel()

	const goroutines = 16

	const iterations = 200

	var wg sync.WaitGroup

	wg.Add(goroutines)

	leaf := errors.New("leaf")

	for range goroutines {
		go func() {
			defer wg.Done()

			for range iterations {
				err := werr.Wrapf(werr.Wrap(leaf), "ctx")
				if !werr.IsWrap(err) {
					panic("Wrap chain must produce a werr.Error")
				}
			}
		}()
	}

	wg.Wait()
}

// scanTraceMarkers returns a map of @trace marker name → 1-indexed line
// number found in the embedded test source. Each marker's line is the
// same line as the wrap call when the marker is a trailing comment.
func scanTraceMarkers(t *testing.T) map[string]int {
	t.Helper()

	markers := make(map[string]int)

	for i, line := range strings.Split(wrapTestSource, "\n") {
		_, after, ok := strings.Cut(line, "// @trace ")
		if !ok {
			continue
		}

		name := strings.TrimSpace(after)
		require.NotEmpty(t, name, "@trace marker on line %d has no name", i+1)
		_, dup := markers[name]
		require.False(t, dup, "@trace name %q appears twice", name)

		markers[name] = i + 1 // 1-indexed
	}

	require.NotEmpty(t, markers, "no @trace markers found in embedded source")

	return markers
}

// captureLine returns the line number of the wrap site stored inside
// err. err must be (or contain) a werr.Error.
func captureLine(t *testing.T, err error) int {
	t.Helper()

	w, ok := werr.AsWrap(err)
	require.True(t, ok, "expected error to be a werr.Error, got: %T %v", err, err)

	return w.Line()
}

// TestWrap_callSites checks that each wrap context captures the correct
// source line. It scans this file for `// @trace <name>` markers and
// matches each against the Line() reported by the resulting werr.Error.
//
// Regression gate for the //go:noinline pragmas on Wrap/Wrapf/Wrap2/Wrap3,
// the asm helper's frame read, and the safe-path skip count — any one of
// those breaking trips most subtests at once.
func TestWrap_callSites(t *testing.T) {
	t.Parallel()

	// Without -race, the Go compiler's inliner sometimes overrides
	// //go:noinline on generic instantiations (Wrap2/Wrap3) and on
	// closures that contain extra basic blocks (anonymous-fn,
	// intermediate-var), which makes the asm PC capture read the wrong
	// frame. Coverage instrumentation amplifies the same issue. -race
	// disables the optimization paths that cause this, so the regression
	// gate stays accurate under CI's `-race -covermode=atomic` and under
	// `make test`. Skip elsewhere so plain `go test` and `go test -cover`
	// stay green for downstream users.
	if !raceEnabled {
		t.Skip("PC-line regression gate requires -race; see `make test` or CI")
	}

	markers := scanTraceMarkers(t)
	leaf := errors.New("leaf")

	type tt struct {
		marker string
		fn     func() error
	}

	plainCases := []tt{
		{"return-wrap", func() error {
			return werr.Wrap(leaf) // @trace return-wrap
		}},
		{"return-wrapf-noargs", func() error {
			return werr.Wrapf(leaf, "ctx") // @trace return-wrapf-noargs
		}},
		{"return-wrapf-fmt", func() error {
			return werr.Wrapf(leaf, "ctx %d", 1) // @trace return-wrapf-fmt
		}},
		{"intermediate-var", func() error {
			wrapped := werr.Wrap(leaf) // @trace intermediate-var

			return wrapped
		}},
		{"in-if", func() error {
			if leaf != nil {
				return werr.Wrap(leaf) // @trace in-if
			}

			return nil
		}},
		{"in-switch", func() error {
			switch leaf {
			case nil:
				return nil
			default:
				return werr.Wrap(leaf) // @trace in-switch
			}
		}},
		{"anonymous-fn", func() error {
			inner := func() error {
				return werr.Wrap(leaf) // @trace anonymous-fn
			}

			return inner()
		}},
		{"defer-wrap", func() (err error) {
			defer func() {
				err = werr.Wrap(err) // @trace defer-wrap
			}()

			return leaf
		}},
		{"nested-wrap-outer", func() error {
			return werr.Wrap(werr.Wrap(leaf)) // @trace nested-wrap-outer
		}},
	}

	for _, tc := range plainCases {
		t.Run(tc.marker, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, markers[tc.marker], captureLine(t, tc.fn()),
				"marker %q expected line %d", tc.marker, markers[tc.marker])
		})
	}

	// Wrap2[T] / Wrap3[A, B] tables — separate to keep type parameters readable.
	t.Run("wrap2-int", func(t *testing.T) {
		t.Parallel()

		_, err := werr.Wrap2(42, leaf) // @trace wrap2-int
		require.Equal(t, markers["wrap2-int"], captureLine(t, err))
	})
	t.Run("wrap2-struct", func(t *testing.T) {
		t.Parallel()

		_, err := werr.Wrap2(struct{ name string }{"x"}, leaf) // @trace wrap2-struct
		require.Equal(t, markers["wrap2-struct"], captureLine(t, err))
	})
	t.Run("wrap2-slice", func(t *testing.T) {
		t.Parallel()

		_, err := werr.Wrap2([]byte{1, 2}, leaf) // @trace wrap2-slice
		require.Equal(t, markers["wrap2-slice"], captureLine(t, err))
	})
	t.Run("wrap3-int-string", func(t *testing.T) {
		t.Parallel()

		_, _, err := werr.Wrap3(42, "alpha", leaf) // @trace wrap3-int-string
		require.Equal(t, markers["wrap3-int-string"], captureLine(t, err))
	})
	t.Run("wrap3-slice-struct", func(t *testing.T) {
		t.Parallel()

		_, _, err := werr.Wrap3([]byte{1}, struct{ x int }{7}, leaf) // @trace wrap3-slice-struct
		require.Equal(t, markers["wrap3-slice-struct"], captureLine(t, err))
	})
}

// TestWrap_callSiteFile covers File() resolution — TestWrap_callSites
// covers Line(). The wrap call sits in a //go:noinline helper so the
// captured frame is unambiguous.
func TestWrap_callSiteFile(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	w, ok := werr.AsWrap(callSiteFileHelper())
	require.True(t, ok)
	require.Equal(t, file, w.File())
}

//go:noinline
func callSiteFileHelper() error {
	return werr.Wrap(errors.New("leaf")) // @trace file-baseline
}
