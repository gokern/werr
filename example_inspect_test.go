package werr_test

import (
	"errors"
	"fmt"
	"io"

	"github.com/gokern/werr"
)

// IsWrap reports whether err's chain contains a werr.Error anywhere,
// including past fmt.Errorf("%w", ...) layers.
func ExampleIsWrap() {
	err := fmt.Errorf("ctx: %w", werr.Wrap(io.EOF))

	fmt.Println(werr.IsWrap(err))

	// Output:
	// true
}

// AsWrap extracts the outermost werr.Error from err's chain. It is the
// idiomatic way to read frame metadata (Message, FuncName, File, Line)
// from a possibly-wrapped error.
func ExampleAsWrap() {
	err := fmt.Errorf("ctx: %w", werr.Wrapf(io.EOF, "boom"))

	if w, ok := werr.AsWrap(err); ok {
		fmt.Println(w.Message())
	}

	// Output:
	// boom
}

// Strip removes one werr layer, exposing the error directly underneath.
// Useful when forwarding to code that does not understand werr — sentinel
// comparisons or concrete-type assertions, for example.
func ExampleStrip() {
	err := werr.Wrap(io.EOF)
	stripped := werr.Strip(err)

	fmt.Println(errors.Is(stripped, io.EOF))

	// Output:
	// true
}

// StripAll removes every consecutive werr layer, returning the first
// non-werr error in the chain. It stops at any non-werr wrapper such as
// fmt.Errorf; for full chain traversal use stdlib errors.Unwrap in a loop.
func ExampleStripAll() {
	err := werr.Wrap(werr.Wrap(werr.Wrap(io.EOF)))
	leaf := werr.StripAll(err)

	fmt.Println(errors.Is(leaf, io.EOF))

	// Output:
	// true
}

// Walk iterates the werr-frames in err's chain from outermost to
// innermost, invoking the callback for each one. Returning false stops
// iteration early. Walk allocates nothing on its iteration path, so it
// is the right primitive for building custom output (Sentry frames,
// structured log fields, and so on).
func ExampleWalk() {
	err := werr.Wrapf(werr.Wrap(werr.Wrapf(io.EOF, "load")), "register")

	var msgs []string

	_ = werr.Walk(err, func(f werr.Frame) bool {
		if f.Msg != "" {
			msgs = append(msgs, f.Msg)
		}

		return true
	})

	fmt.Println(msgs)

	// Output:
	// [register load]
}
