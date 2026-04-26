package sentry_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	sentry "github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/require"

	"github.com/gokern/werr"
)

// makeInner / makeMiddle / makeOuter are the same helpers used in core
// stacktrace_test.go. Each gets its own PC so the resolved sentry
// frames are distinguishable; without //go:noinline they would collapse.
//
//go:noinline so each wrap has a real stack frame for the asm pc.Caller path.
func makeInner(leaf error) error { return werr.Wrap(leaf) }

//go:noinline so each wrap has a real stack frame for the asm pc.Caller path.
func makeMiddle(leaf error) error { return werr.Wrapf(makeInner(leaf), "middle") }

//go:noinline so each wrap has a real stack frame for the asm pc.Caller path.
func makeOuter(leaf error) error { return werr.Wrapf(makeMiddle(leaf), "outer") }

// Eight wrap helpers, each at a distinct source line. A naive
// `for range 8 { err = werr.Wrap(err) }` produces eight identical PCs
// (same call site) that Callers happily collects but Sentry renders as
// eight indistinguishable rows. Real wrap chains scatter across
// functions, so the test should too.

//go:noinline
func makeD1(leaf error) error { return werr.Wrap(leaf) }

//go:noinline
func makeD2(leaf error) error { return werr.Wrap(makeD1(leaf)) }

//go:noinline
func makeD3(leaf error) error { return werr.Wrap(makeD2(leaf)) }

//go:noinline
func makeD4(leaf error) error { return werr.Wrap(makeD3(leaf)) }

//go:noinline
func makeD5(leaf error) error { return werr.Wrap(makeD4(leaf)) }

//go:noinline
func makeD6(leaf error) error { return werr.Wrap(makeD5(leaf)) }

//go:noinline
func makeD7(leaf error) error { return werr.Wrap(makeD6(leaf)) }

//go:noinline
func makeD8(leaf error) error { return werr.Wrap(makeD7(leaf)) }

func TestExtractStacktrace_returnsNilForNonWerr(t *testing.T) {
	t.Parallel()

	// No nil case here. sentry-go's ExtractStacktrace(nil) panics inside
	// the SDK; real callers gate with `if err != nil` before reporting.

	require.Nil(t, sentry.ExtractStacktrace(io.EOF),
		"foreign error types must not activate werr's protocol")
	require.Nil(t, sentry.ExtractStacktrace(errors.New("plain")),
		"plain stdlib errors yield no stacktrace")
}

func TestExtractStacktrace_returnsNonNilForWerrChain(t *testing.T) {
	t.Parallel()

	err := makeOuter(io.EOF)

	st := sentry.ExtractStacktrace(err)
	require.NotNil(t, st, "sentry must discover StackTrace() on *werr.Error")
	require.NotEmpty(t, st.Frames)
}

func TestExtractStacktrace_frameCountMatchesChainDepth(t *testing.T) {
	t.Parallel()

	t.Run("depth 1", func(t *testing.T) {
		t.Parallel()
		err := werr.Wrap(io.EOF)
		st := sentry.ExtractStacktrace(err)
		require.NotNil(t, st)
		require.Len(t, st.Frames, 1)
	})

	t.Run("depth 3", func(t *testing.T) {
		t.Parallel()
		err := makeOuter(io.EOF)
		st := sentry.ExtractStacktrace(err)
		require.NotNil(t, st)
		require.Len(t, st.Frames, 3)
	})

	t.Run("depth 8 with distinct PCs", func(t *testing.T) {
		t.Parallel()
		err := makeD8(errors.New("leaf"))

		st := sentry.ExtractStacktrace(err)
		require.NotNil(t, st)
		require.Len(t, st.Frames, 8)

		// Each frame must have a unique function name; duplicates would
		// mean distinct PCs collapsed and the dashboard shows N copies
		// of the same row.
		seen := make(map[string]struct{}, len(st.Frames))
		for i, f := range st.Frames {
			_, dup := seen[f.Function]
			require.False(t, dup,
				"Frames[%d].Function %q already seen, distinct PCs collapsed",
				i, f.Function)
			seen[f.Function] = struct{}{}
		}
	})
}

func TestExtractStacktrace_orderMatchesSentryConvention(t *testing.T) {
	t.Parallel()

	// sentry.Stacktrace.Frames is documented as "in reverse order, with
	// the most recent call last", so Frames[0] is closest to the program
	// entrypoint and Frames[len-1] is closest to the failure. Callers
	// emits innermost-first; sentry-go's extractFrames flips it. We
	// assert against the post-flip layout that consumers see.
	err := makeOuter(io.EOF)

	st := sentry.ExtractStacktrace(err)
	require.NotNil(t, st)
	require.Len(t, st.Frames, 3)

	require.True(t, strings.HasSuffix(st.Frames[0].Function, "makeOuter"),
		"Frames[0] must be the outermost wrap (most recent call, post-flip first); got %q", st.Frames[0].Function)
	require.True(t, strings.HasSuffix(st.Frames[1].Function, "makeMiddle"),
		"got %q", st.Frames[1].Function)
	require.True(t, strings.HasSuffix(st.Frames[2].Function, "makeInner"),
		"got %q", st.Frames[2].Function)
}

func TestExtractStacktrace_frameContentsMatchSourceSites(t *testing.T) {
	t.Parallel()

	// makeInner / makeMiddle / makeOuter live at fixed lines in this file.
	// Check that sentry's resolved Frame has the right Function name and
	// a non-zero Lineno. Filename is checked loosely (must contain
	// "sentry_test.go") because the runtime returns absolute or
	// trim-pathed paths depending on build flags.
	err := makeOuter(io.EOF)

	st := sentry.ExtractStacktrace(err)
	require.NotNil(t, st)
	require.Len(t, st.Frames, 3)

	for i, f := range st.Frames {
		require.NotEmpty(t, f.Function, "Frames[%d].Function must be non-empty", i)
		require.NotZero(t, f.Lineno, "Frames[%d].Lineno must be non-zero", i)

		hasFile := f.Filename != "" || f.AbsPath != ""
		require.True(t, hasFile, "Frames[%d] must carry Filename or AbsPath", i)

		if f.Filename != "" {
			require.Contains(t, f.Filename, "sentry_test.go",
				"Frames[%d].Filename must reference the wrap-site source", i)
		}

		if f.AbsPath != "" {
			require.Contains(t, f.AbsPath, "sentry_test.go",
				"Frames[%d].AbsPath must reference the wrap-site source", i)
		}
	}
}

func TestExtractStacktrace_throughFmtErrorf(t *testing.T) {
	t.Parallel()

	// werr -> fmt.Errorf -> werr -> io.EOF. sentry asks the outermost
	// link for StackTrace(); Callers walks to the fmt.Errorf boundary
	// and stops, returning only the outer PC. Sentry should see one
	// frame.
	inner := werr.Wrap(io.EOF)
	mid := fmt.Errorf("mid: %w", inner)
	outer := werr.Wrapf(mid, "outer")

	st := sentry.ExtractStacktrace(outer)
	require.NotNil(t, st)
	require.Len(t, st.Frames, 1, "Callers stops at fmt.Errorf, so only the outer werr-frame survives")
}

func TestExtractStacktrace_outerNotWerrYieldsNil(t *testing.T) {
	t.Parallel()

	// Outer link is *fmt.wrapError, not *werr.Error. sentry's
	// MethodByName("StackTrace") miss makes ExtractStacktrace return nil;
	// werr does not provide a stack from underneath foreign wrappers.
	wrapped := fmt.Errorf("crossing %w", werr.Wrap(io.EOF))

	require.Nil(t, sentry.ExtractStacktrace(wrapped))
}

func TestExtractStacktrace_jsonRoundTrip(t *testing.T) {
	t.Parallel()

	err := makeOuter(io.EOF)

	original := sentry.ExtractStacktrace(err)
	require.NotNil(t, original)

	raw, mErr := json.Marshal(original)
	require.NoError(t, mErr)
	require.Contains(t, string(raw), "makeInner",
		"serialised stacktrace must include resolved function names")

	var roundTripped sentry.Stacktrace
	require.NoError(t, json.Unmarshal(raw, &roundTripped))

	require.Len(t, roundTripped.Frames, len(original.Frames))
	for i := range original.Frames {
		require.Equal(t, original.Frames[i].Function, roundTripped.Frames[i].Function)
		require.Equal(t, original.Frames[i].Lineno, roundTripped.Frames[i].Lineno)
	}
}
