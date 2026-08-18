package werr_test

import (
	"fmt"
	"io"
	"strings"

	"github.com/gokern/werr/v2"
)

// Pretty renders an error chain as a multi-line stack trace, regardless of
// what is currently installed via [SetFormatter]. Use it for spot
// rendering at a single log site.
func ExamplePretty() {
	err := werr.Wrapf(io.EOF, "loading config")

	out := werr.Pretty(err)
	// The output starts with the outermost message and ends with the leaf
	// (file paths and line numbers are runtime-dependent).
	fmt.Println(strings.HasPrefix(out, "loading config\n"))
	fmt.Println(strings.HasSuffix(out, "Caused by: EOF"))

	// Output:
	// true
	// true
}

// OneLine renders an error chain as a single line suitable for log
// aggregators (Loki, ELK) and grep-style tooling. It guarantees no
// newlines, tabs, or carriage returns in the output, even if user
// messages contain them.
func ExampleOneLine() {
	err := werr.Wrapf(io.EOF, "loading config")

	out := werr.OneLine(err)
	fmt.Println(strings.Contains(out, "\n"))
	fmt.Println(strings.HasSuffix(out, " -> EOF"))

	// Output:
	// false
	// true
}

// SetFormatter installs the active formatter used by every Error.Error()
// call. Typical usage: install once from application init based on the
// deployment environment, then everywhere fmt.Println(err) /
// log.Println(err) / sentry.Capture(err) produces consistent output.
//
// SetFormatter(nil) is a no-op (preserves the current setting).
func ExampleSetFormatter() {
	// Install a tiny deterministic formatter just for this example so the
	// output is verifiable. Production code would use SetPrettyFormatter,
	// SetOneLineFormatter, or SetFormatter with a custom FormatFn.
	werr.SetFormatter(func(frames []werr.Frame, leaf error) string {
		var sb strings.Builder

		for _, f := range frames {
			if f.Msg != "" {
				sb.WriteString(f.Msg)
				sb.WriteString(" / ")
			}
		}

		if leaf != nil {
			sb.WriteString(leaf.Error())
		}

		return sb.String()
	})

	defer werr.SetPrettyFormatter() // restore default

	err := werr.Wrapf(werr.Wrapf(io.EOF, "inner"), "outer")
	fmt.Println(err)

	// Output:
	// outer / inner / EOF
}
