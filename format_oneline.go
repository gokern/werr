package werr

import (
	"path"
	"strconv"
	"strings"

	"github.com/gokern/werr/internal/funcname"
)

// OneLineSeparator is the separator [OneLineFormatter] places between
// segments. Exposed so callers can split the output:
//
//	parts := strings.Split(werr.OneLine(err), werr.OneLineSeparator)
const OneLineSeparator = " -> "

// OneLineFormatter renders the chain on a single line for log aggregators
// (Loki, ELK) and grep-friendly tooling:
//
//	register at discovery.go:265 (reallyRegister) -> impl.go:89 (Register) -> load at config.go:44 (with) -> config missing
//
// Each werr frame is one segment; segments are joined with [OneLineSeparator].
// A frame with non-empty Msg renders as "<msg> at <basename>:<line> (<func>)";
// a frame without Msg renders as "<basename>:<line> (<func>)". The leaf
// error text is the final segment.
//
// Output never contains \n, \r, or \t. User-supplied Msg and the leaf
// error text are flattened so line-based parsers stay intact.
//
// Install as the global formatter via [SetOneLineFormatter], or call
// [OneLine] for one-off rendering. OneLineFormatter itself is exposed for
// composition:
//
//	werr.SetFormatter(func(frames []werr.Frame, leaf error) string {
//	    return "[APP] " + werr.OneLineFormatter(frames, leaf)
//	})
func OneLineFormatter(frames []Frame, leaf error) string {
	if len(frames) == 0 {
		if leaf == nil {
			return ""
		}

		return flattenIfNeeded(leaf.Error())
	}

	var sb strings.Builder

	sb.Grow(oneLineEstimate(frames, leaf))

	for i, f := range frames {
		if i > 0 {
			sb.WriteString(OneLineSeparator)
		}

		writeOneLineFrame(&sb, f)
	}

	if leaf != nil {
		sb.WriteString(OneLineSeparator)
		writeFlattened(&sb, leaf.Error())
	}

	return sb.String()
}

func writeOneLineFrame(sb *strings.Builder, frame Frame) {
	_, fn := funcname.Split(frame.FuncName)

	if frame.Msg != "" {
		writeFlattened(sb, frame.Msg)
		sb.WriteString(" at ")
	}

	sb.WriteString(path.Base(frame.File))

	if frame.Line > 0 {
		sb.WriteByte(':')
		sb.WriteString(strconv.Itoa(frame.Line))
	}

	sb.WriteString(" (")
	sb.WriteString(fn)
	sb.WriteByte(')')
}

// writeFlattened writes s into sb, replacing \n, \r, and \t with a single
// space. The fast path (no whitespace control chars) is one WriteString.
func writeFlattened(sb *strings.Builder, str string) {
	i := strings.IndexAny(str, "\n\r\t")
	if i < 0 {
		sb.WriteString(str)

		return
	}

	sb.WriteString(str[:i])

	for j := i; j < len(str); j++ {
		c := str[j]
		if c == '\n' || c == '\r' || c == '\t' {
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(c)
		}
	}
}

func flattenIfNeeded(str string) string {
	if !strings.ContainsAny(str, "\n\r\t") {
		return str
	}

	var b strings.Builder

	b.Grow(len(str))
	writeFlattened(&b, str)

	return b.String()
}

func oneLineEstimate(frames []Frame, leaf error) int {
	const leafReserve = 64

	const framePrefix = len(OneLineSeparator) + len(" at ") + len(":") + len(" (") + len(")") + 8

	size := 0
	for _, f := range frames {
		size += framePrefix + len(f.Msg) + len(f.File) + len(f.FuncName)
	}

	if leaf != nil {
		size += len(OneLineSeparator) + leafReserve
	}

	return size
}
