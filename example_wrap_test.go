package werr_test

import (
	"errors"
	"fmt"
	"io"

	"github.com/gokern/werr"
)

// Wrap adds a single werr layer to err, capturing the file, line and
// function of the caller. Wrap(nil) returns nil so it is safe to write
// `return werr.Wrap(err)` unconditionally.
func ExampleWrap() {
	err := werr.Wrap(io.EOF)

	// Identity through the layer is preserved for errors.Is.
	fmt.Println(errors.Is(err, io.EOF))

	// Output:
	// true
}

// Wrap returns nil unchanged, so `return werr.Wrap(err)` is safe
// regardless of whether err is nil.
func ExampleWrap_nil() {
	fmt.Println(werr.Wrap(nil) == nil)

	// Output:
	// true
}

// Wrapf adds a layer with a printf-style message attached. The message
// becomes the heading rendered by [PrettyFormatter] / [OneLineFormatter]
// and is exposed via [Error.Message].
func ExampleWrapf() {
	err := werr.Wrapf(io.EOF, "reading %s (offset %d)", "config.yaml", 0)

	w, _ := werr.AsWrap(err)
	fmt.Println(w.Message())

	// Output:
	// reading config.yaml (offset 0)
}

// Wrap2 forwards the common (T, error) tuple, adding a werr layer if
// the error is non-nil.
func ExampleWrap2() {
	makeUser := func() (string, error) {
		return "alice", io.EOF
	}

	name, err := werr.Wrap2(makeUser())
	fmt.Println(name)
	fmt.Println(errors.Is(err, io.EOF))

	// Output:
	// alice
	// true
}
