package werr

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetFormatter(t *testing.T) {
	t.Cleanup(SetPrettyFormatter)

	t.Run("custom formatter is used by Error()", func(t *testing.T) {
		custom := FormatFn(func(_ []Frame, leaf error) string {
			return "custom:" + leaf.Error()
		})
		SetFormatter(custom)

		err := Wrap(errors.New("leaf"))
		require.Equal(t, "custom:leaf", err.Error())
	})

	t.Run("nil is a no-op (preserves current formatter)", func(t *testing.T) {
		marker := FormatFn(func(_ []Frame, leaf error) string { return "M:" + leaf.Error() })
		SetFormatter(marker)

		SetFormatter(nil) // must not reset

		err := Wrap(errors.New("leaf"))
		require.Equal(t, "M:leaf", err.Error())
	})
}

func TestBuiltinFormatters(t *testing.T) {
	t.Cleanup(SetPrettyFormatter)

	t.Run("PrettyFormatter installed by default", func(t *testing.T) {
		err := Wrap(errors.New("leaf"))
		out := err.Error()
		require.Contains(t, out, "leaf")
		require.Contains(t, out, " --- at ")
	})

	t.Run("SetOneLineFormatter selects single-line output", func(t *testing.T) {
		SetOneLineFormatter()

		err := Wrap(errors.New("leaf"))
		out := err.Error()
		require.NotContains(t, out, "\n")
		require.Contains(t, out, OneLineSeparator)
	})

	t.Run("SetPrettyFormatter restores multi-line output", func(t *testing.T) {
		SetPrettyFormatter()

		err := Wrap(errors.New("leaf"))
		out := err.Error()
		require.Contains(t, out, " --- at ")
	})
}

// White-box tests for the two PrettyFormatter branches that the public
// SetPretty / Pretty paths don't naturally exercise: headingFromLeaf=true
// (outermost frame has no Msg, so leaf text becomes the heading) and a
// chain whose consecutive frames carry the same Msg (suppresses the
// repeated "Caused by:" line).
func TestPrettyFormatter_branches(t *testing.T) {
	t.Parallel()

	t.Run("empty outer Msg promotes leaf text to heading", func(t *testing.T) {
		t.Parallel()

		// Outer Wrap has no Msg, inner Wrapf does. With headingFromLeaf=true
		// the leaf ("leaf") is the heading and the trailing "Caused by:"
		// for the leaf is suppressed.
		err := Wrap(Wrapf(errors.New("leaf"), "inner"))
		out := err.Error()

		require.True(t, strings.HasPrefix(out, "leaf\n"),
			"empty outermost Msg must put the leaf text on the heading line")
		require.Contains(t, out, "Caused by: inner",
			"the inner Msg must still appear as a Caused by: line")
		require.Equal(t, 1, strings.Count(out, "Caused by:"),
			"trailing leaf Caused by: must be suppressed when leaf is already the heading")
	})

	t.Run("consecutive frames with the same Msg do not repeat Caused by", func(t *testing.T) {
		t.Parallel()

		// Two outer Wrapf with identical messages: the second frame must
		// attach to the existing heading without producing a second
		// "Caused by: same" line.
		err := Wrapf(Wrapf(errors.New("leaf"), "same"), "same")
		out := err.Error()

		require.True(t, strings.HasPrefix(out, "same\n"))
		// Only the leaf gets a "Caused by:" — the duplicated outer Msg does not.
		require.Equal(t, 1, strings.Count(out, "Caused by:"),
			"repeated identical Msg must not emit a second Caused by:")
		require.Contains(t, out, "Caused by: leaf")
	})
}

// A panic inside a user-installed FormatFn must propagate out of
// Error.Error() unchanged. werr deliberately does not wrap formatter
// panics in a recover — silently swallowing the panic would hide the
// defect and emit an empty/garbled string in its place.
func TestSetFormatter_userPanicPropagates(t *testing.T) {
	t.Cleanup(SetPrettyFormatter)

	SetFormatter(func(_ []Frame, _ error) string {
		panic("formatter intentionally panicked")
	})

	err := Wrap(errors.New("leaf"))

	require.PanicsWithValue(t, "formatter intentionally panicked", func() {
		_ = err.Error()
	})
}

// Concurrent writers and readers — a non-atomic install would trip -race.
func TestSetFormatter_concurrent(t *testing.T) {
	t.Cleanup(SetPrettyFormatter)

	const goroutines = 16

	const iterations = 1000

	custom := FormatFn(func(_ []Frame, leaf error) string { return "C:" + leaf.Error() })

	var wg sync.WaitGroup

	wg.Add(goroutines * 2)

	for range goroutines {
		go func() {
			defer wg.Done()

			for j := range iterations {
				if j%2 == 0 {
					SetFormatter(custom)
				} else {
					SetPrettyFormatter()
				}
			}
		}()

		go func() {
			defer wg.Done()

			err := Wrap(errors.New("leaf"))
			for range iterations {
				_ = err.Error()
			}
		}()
	}

	wg.Wait()
}

func TestHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Pretty matches default Error() output", func(t *testing.T) {
		t.Parallel()

		err := Wrapf(errors.New("leaf"), "ctx")
		require.Contains(t, Pretty(err), "ctx")
		require.Contains(t, Pretty(err), "leaf")
		require.Contains(t, Pretty(err), " --- at ")
	})

	t.Run("OneLine produces single-line output", func(t *testing.T) {
		t.Parallel()

		err := Wrapf(errors.New("leaf"), "ctx")
		out := OneLine(err)
		require.NotContains(t, out, "\n")
		require.True(t, strings.HasPrefix(out, "ctx at "))
		require.True(t, strings.HasSuffix(out, " -> leaf"))
	})

	t.Run("Pretty(nil) returns empty string", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, Pretty(nil))
	})

	t.Run("OneLine(nil) returns empty string", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, OneLine(nil))
	})

	t.Run("non-werr error renders via underlying Error()", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "plain", Pretty(errors.New("plain")))
		require.Equal(t, "plain", OneLine(errors.New("plain")))
	})
}

// FuzzPretty: never panics, always contains the leaf message.
func FuzzPretty(f *testing.F) {
	f.Add("leaf", "msg", uint8(1))
	f.Add("", "", uint8(0))
	f.Add("multi\nline", "with\ttabs", uint8(3))
	f.Add("\x00binary", "🚀 emoji", uint8(7))

	f.Fuzz(func(t *testing.T, leafMsg, wrapMsg string, depth uint8) {
		err := errors.New(leafMsg)

		for range int(depth) % 16 {
			if wrapMsg == "" {
				err = Wrap(err)
			} else {
				err = Wrapf(err, "%s", wrapMsg)
			}
		}

		out := Pretty(err)
		require.Contains(t, out, leafMsg)
	})
}

// FuzzOneLine asserts the single-line invariant for any input.
func FuzzOneLine(f *testing.F) {
	f.Add("leaf", "msg", uint8(1))
	f.Add("multi\nline\rleaf", "tabbed\tmsg", uint8(3))
	f.Add("\x00binary", "🚀 emoji", uint8(7))

	f.Fuzz(func(t *testing.T, leafMsg, wrapMsg string, depth uint8) {
		err := errors.New(leafMsg)

		for range int(depth) % 16 {
			if wrapMsg == "" {
				err = Wrap(err)
			} else {
				err = Wrapf(err, "%s", wrapMsg)
			}
		}

		out := OneLine(err)
		require.NotContains(t, out, "\n")
		require.NotContains(t, out, "\r")
		require.NotContains(t, out, "\t")
	})
}
