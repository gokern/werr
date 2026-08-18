package werr

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gokern/panics"
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

// SetOneLineFormatter writes only _kind, leaving _formatter stale —
// distinct write pattern from SetFormatter, so race it explicitly.
func TestSetOneLineFormatter_concurrent(t *testing.T) {
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
				switch j % 3 {
				case 0:
					SetOneLineFormatter()
				case 1:
					SetFormatter(custom)
				default:
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

// Inline render buffers in Error.Error / Pretty / OneLine are sized [16];
// chains beyond that overflow to heap. Verify output stays correct.
func TestRender_chainDeeperThan16(t *testing.T) {
	t.Parallel()

	const depth = 20

	var err = errors.New("leaf")
	for i := range depth {
		err = Wrapf(err, "frame-%d", i)
	}

	t.Run("Error()", func(t *testing.T) {
		t.Parallel()

		out := err.Error()
		require.Contains(t, out, "leaf")
		require.Contains(t, out, "frame-0")
		require.Contains(t, out, "frame-19")
		require.Equal(t, depth, strings.Count(out, " --- at "))
	})

	t.Run("Pretty", func(t *testing.T) {
		t.Parallel()

		out := Pretty(err)
		require.Contains(t, out, "leaf")
		require.Contains(t, out, "frame-19")
		require.Equal(t, depth, strings.Count(out, " --- at "))
	})

	t.Run("OneLine", func(t *testing.T) {
		t.Parallel()

		out := OneLine(err)
		require.NotContains(t, out, "\n")
		require.Contains(t, out, "leaf")
		require.Contains(t, out, "frame-19")
	})
}

// Empty leaf message must not produce a leading bare newline in
// PrettyFormatter output (regression: errors.New("") wrapped by Wrap
// previously rendered as "\n --- at ...").
func TestPrettyFormatter_emptyLeaf(t *testing.T) {
	t.Parallel()

	out := Pretty(Wrap(errors.New("")))
	require.False(t, strings.HasPrefix(out, "\n"),
		"empty heading must not produce a leading newline; got %q", out)
	require.Contains(t, out, " --- at ")
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

// The single render allocation must not grow with the depth of the checkout.
// Both estimates used to reserve len(f.File) while the writers emit
// path.Base(f.File), so identical output cost 2304 B/op from one tree and
// 2688 B/op from another twenty characters deeper — and the numbers quoted in
// CLAUDE.md were only reproducible on a path of the same length as the one
// they were measured on.
//
// Not parallel: AllocsPerRun panics if it is.
func TestFormatEstimates_doNotScaleWithCheckoutDepth(t *testing.T) {
	const depth = 15

	shallow := make([]Frame, depth)
	deep := make([]Frame, depth)

	for i := range shallow {
		shallow[i] = Frame{
			File:     "/w/service.go",
			Line:     42,
			FuncName: "github.com/gokern/werr/v2.Handle",
			Msg:      "handling request",
		}

		deep[i] = shallow[i]
		deep[i].File = "/home/runner/work/repo/repo/internal/service/service.go"
	}

	leaf := errors.New("leaf")
	heading := shallow[0].Msg

	require.Equal(t,
		prettyEstimate(shallow, heading, "leaf", false),
		prettyEstimate(deep, heading, "leaf", false),
		"a deeper checkout must not enlarge the pretty reservation")
	require.Equal(t,
		oneLineEstimate(shallow, leaf),
		oneLineEstimate(deep, leaf),
		"nor the one-line reservation")

	require.Equal(t, PrettyFormatter(shallow, leaf), PrettyFormatter(deep, leaf),
		"the rendered output was identical all along; only the reservation moved")

	// The estimate is still an upper bound, or the Builder reallocates and
	// the one-allocation render quietly becomes two.
	prettyAllocs := testing.AllocsPerRun(100, func() { _ = PrettyFormatter(deep, leaf) })
	require.LessOrEqual(t, prettyAllocs, 1.0,
		"pretty render must stay a single allocation on a deep path; got %.2f", prettyAllocs)

	oneLineAllocs := testing.AllocsPerRun(100, func() { _ = OneLineFormatter(deep, leaf) })
	require.LessOrEqual(t, oneLineAllocs, 1.0,
		"one-line render must stay a single allocation on a deep path; got %.2f", oneLineAllocs)
}

//go:noinline
func raisePanicForFormat() { panic("boom") }

// caughtPanic is the shape werr receives: something else contained the panic,
// and werr is handed the result to wrap.
//
//nolint:wrapcheck // the point is the raw *panics.Panic, unwrapped, as a producer hands it over.
func caughtPanic() error { return panics.Catch(raisePanicForFormat) }

func TestPrettyFormatter_expandsAPanicLeaf(t *testing.T) {
	out := Pretty(Wrap(caughtPanic()))

	require.Contains(t, out, "panic: boom", "the panic value heads the output")
	require.Contains(t, out, "raisePanicForFormat",
		"the panic site must be rendered, not just the catch site")
	require.Contains(t, out, " --- at ",
		"panic frames use the same shape as wrap frames")
	require.NotContains(t, out, "\n\n",
		"panic frames continue the one frame list; a blank line would re-announce "+
			"the seam the shared shape exists to hide")
}

// The other half of writePanicFrames' lineOpen: when the outermost frame
// carries a Msg, the leaf renders as a "Caused by:" tail that writePrettyFrame
// never terminated, so the panic frames must supply the newline themselves.
// Asserting the exact junction pins both branches — drop the newline and the
// Contains fails, emit it unconditionally and the NotContains does.
func TestPrettyFormatter_expandsAPanicLeafUnderAWrap(t *testing.T) {
	out := Pretty(Wrapf(caughtPanic(), "handling request"))

	require.Contains(t, out, "\nCaused by: panic: boom\n --- at ")
	require.NotContains(t, out, "\n\n")
}

// The panic that reaches a formatter in production is rarely the bare leaf:
// taskgroup joins them, and the idiom panics recommends is fmt.Errorf with a
// sentinel. Matching the leaf by type covered neither, which is why both
// formatters go through panics.As.
func TestPrettyFormatter_findsAPanicBuriedInTheChain(t *testing.T) {
	t.Parallel()

	errSentinel := errors.New("delivery failed")

	for name, leaf := range map[string]error{
		"errors.Join":       errors.Join(caughtPanic()),
		"errors.Join pair":  errors.Join(caughtPanic(), errors.New("other")),
		"fmt.Errorf %w":     fmt.Errorf("task %q: %w", "sync", caughtPanic()),
		"fmt.Errorf two %w": fmt.Errorf("%w: %w", errSentinel, caughtPanic()),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Contains(t, Pretty(Wrapf(leaf, "handling request")), "raisePanicForFormat",
				"the panic site must survive a chain that buried the panic")
		})
	}
}

// A panic does not need a werr layer above it to be rendered. panics ships no
// formatters and assigns rendering here, so if these two channels skip the
// stack when no wrap frame is present, nothing in the pair prints it — and the
// shapes below are the ones a panic actually arrives in, including the
// fmt.Errorf idiom panics' own README recommends.
func TestPrettyFormatter_expandsAPanicWithNoWrapFrames(t *testing.T) {
	t.Parallel()

	errSentinel := errors.New("delivery failed")

	for name, err := range map[string]error{
		"bare":              caughtPanic(),
		"errors.Join":       errors.Join(caughtPanic()),
		"errors.Join pair":  errors.Join(caughtPanic(), errors.New("other")),
		"fmt.Errorf %w":     fmt.Errorf("task %q: %w", "sync", caughtPanic()),
		"fmt.Errorf two %w": fmt.Errorf("%w: %w", errSentinel, caughtPanic()),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out := Pretty(err)

			require.Contains(t, out, "panic: boom", "the leaf text still heads the output")
			require.Contains(t, out, "raisePanicForFormat",
				"an unwrapped panic must render its site; Pretty(p) and Pretty(Wrap(p)) "+
					"cannot disagree on whether the stack is visible")
			require.Contains(t, out, " --- at ",
				"panic frames keep the shape they have under a wrap")
			require.NotContains(t, out, "\n\n",
				"the leaf line and the frame list are one block")
		})
	}
}

func TestOneLineFormatter_namesThePanicSiteWithNoWrapFrames(t *testing.T) {
	t.Parallel()

	out := OneLine(caughtPanic())

	require.Contains(t, out, "panic: boom")
	require.Contains(t, out, "panic at ")
	require.Contains(t, out, "raisePanicForFormat")
	require.Equal(t, 1, strings.Count(out, "panic at "), "still exactly one segment")

	require.NotContains(t, out, "\n", "the single-line guarantee holds on this path too")
	require.NotContains(t, out, "\r")
	require.NotContains(t, out, "\t")
}

// The other half of the frameless branch: an ordinary error must come back
// byte-identical and, for Pretty, without touching the Builder at all. This is
// what the panics.As lookup is allowed to cost — one failed assertion, no
// allocation, no formatting.
// Not parallel: AllocsPerRun panics if it is.
func TestFormatters_leafOnlyPathIsUnchangedForOrdinaryErrors(t *testing.T) {
	leaf := errors.New("plain leaf")

	require.Equal(t, "plain leaf", Pretty(leaf))
	require.Equal(t, "plain leaf", OneLine(leaf))
	require.Empty(t, Pretty(nil))
	require.Empty(t, OneLine(nil))

	require.Equal(t, "a b c", OneLine(errors.New("a\nb\tc")),
		"flattening still applies with no frames above the leaf")

	allocs := testing.AllocsPerRun(100, func() {
		_ = Pretty(leaf)
	})
	require.Zero(t, allocs, "an ordinary error must not pay for the panic lookup")
}

// OneLine names the site and stops: a format whose contract is "one record is
// one line" cannot carry a 60-frame stack.
func TestOneLineFormatter_namesThePanicSiteOnly(t *testing.T) {
	out := OneLine(Wrap(caughtPanic()))

	require.Contains(t, out, "panic: boom")
	require.Contains(t, out, "panic at ", "the segment must label itself as a site")
	require.Contains(t, out, "raisePanicForFormat", "and name where the panic was raised")

	require.NotContains(t, out, "runtime.goexit",
		"only the panic site is emitted, never the rest of the goroutine stack")
	require.Equal(t, 1, strings.Count(out, "panic at "),
		"exactly one panic segment, however deep the stack")

	require.NotContains(t, out, "\n", "the single-line guarantee is absolute")
	require.NotContains(t, out, "\r")
	require.NotContains(t, out, "\t")
}
