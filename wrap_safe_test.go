//go:build werrsafe

package werr_test

import (
	_ "embed"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gokern/werr"
)

//go:embed wrap_safe_test.go
var wrapSafeTestSource string

// scanSafeTraceMarkers is scanTraceMarkers's werrsafe-build companion;
// it scans this file's source so marker names don't collide with the
// asm-build scanner.
func scanSafeTraceMarkers(t *testing.T) map[string]int {
	t.Helper()
	markers := make(map[string]int)
	for i, line := range strings.Split(wrapSafeTestSource, "\n") {
		idx := strings.Index(line, "// @trace ")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[idx+len("// @trace "):])
		require.NotEmpty(t, name, "@trace marker on line %d has no name", i+1)
		markers[name] = i + 1
	}
	require.NotEmpty(t, markers, "no @trace markers found in embedded source")
	return markers
}

// TestWrap_callSites_safe is TestWrap_callSites's werrsafe-build
// companion. Each scans its own source under the matching build tag,
// so the asm and safe paths are checked against the same property
// independently.
func TestWrap_callSites_safe(t *testing.T) {
	t.Parallel()

	markers := scanSafeTraceMarkers(t)
	leaf := errors.New("leaf")

	captureLine := func(err error) int {
		w, ok := werr.AsWrap(err)
		require.True(t, ok)
		return w.Line()
	}

	tests := []struct {
		marker string
		fn     func() error
	}{
		{"safe-return-wrap", func() error {
			return werr.Wrap(leaf) // @trace safe-return-wrap
		}},
		{"safe-return-wrapf", func() error {
			return werr.Wrapf(leaf, "ctx %d", 1) // @trace safe-return-wrapf
		}},
		{"safe-defer-wrap", func() (err error) {
			defer func() {
				err = werr.Wrap(err) // @trace safe-defer-wrap
			}()
			return leaf
		}},
		{"safe-anonymous-fn", func() error {
			inner := func() error {
				return werr.Wrap(leaf) // @trace safe-anonymous-fn
			}
			return inner()
		}},
		{"safe-nested", func() error {
			return werr.Wrap(werr.Wrap(leaf)) // @trace safe-nested
		}},
	}

	for _, tc := range tests {
		t.Run(tc.marker, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, markers[tc.marker], captureLine(tc.fn()))
		})
	}

	t.Run("safe-wrap2-int", func(t *testing.T) {
		t.Parallel()
		_, err := werr.Wrap2(7, leaf) // @trace safe-wrap2-int
		require.Equal(t, markers["safe-wrap2-int"], captureLine(err))
	})

	t.Run("safe-wrap3-int-string", func(t *testing.T) {
		t.Parallel()
		_, _, err := werr.Wrap3(7, "x", leaf) // @trace safe-wrap3-int-string
		require.Equal(t, markers["safe-wrap3-int-string"], captureLine(err))
	})
}
