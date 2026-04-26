package werr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCallers_skipsZeroPC builds a chain with a synthetic pc==0 link.
// pc==0 only happens if the safe-fallback runtime.Callers returns
// nothing (basically never), but Callers must drop that frame instead
// of leaving a zero hole in the output.
func TestCallers_skipsZeroPC(t *testing.T) {
	t.Parallel()

	leaf := errors.New("leaf")

	innerWithZero := newError(leaf, "inner-zero", 0)

	middle := newError(innerWithZero, "middle", 0xCAFE)
	outer := newError(middle, "outer", 0xBEEF)

	pcs := Callers(outer)

	require.Equal(t, []uintptr{0xCAFE, 0xBEEF}, pcs,
		"zero-PC frame must be skipped, surviving frames keep their order (innermost-first)")
}
