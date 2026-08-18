package werr

import (
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/gokern/werr/v2/internal/funcname"
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
// When the leaf carries a recovered panic, one further segment names where it
// was raised: "panic at <basename>:<line> (<func>)". Only that frame — the
// rest of the stack stays out. A one-line format exists so that a record is a
// line; spilling a 60-frame stack into it is the thing panics keeps the stack
// out of its message to avoid. Reach the full stack through panics.As, or read
// it off the [PrettyFormatter] output.
//
// The segment does not need a wrap above it: a panic returned without a werr
// layer still names its site.
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
		return oneLineLeafOnly(leaf)
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

		if stack := panicStack(leaf); stack != nil {
			writeOneLinePanicSite(&sb, stack)
		}
	}

	return sb.String()
}

// oneLineLeafOnly renders a leaf that has no werr layer above it: its text,
// flattened, which is what the single-line guarantee rests on — plus the
// panic segment when it carries one. See prettyLeafOnly for why an unwrapped
// panic is still werr's to render.
func oneLineLeafOnly(leaf error) string {
	if leaf == nil {
		return ""
	}

	msg := leaf.Error()

	stack := panicStack(leaf)
	if stack == nil {
		return flattenIfNeeded(msg)
	}

	// One segment's worth: the separator, "panic at ", a base file name, a
	// line number and a bare function name. Typical, not guaranteed — an
	// unusually long file or function name costs one Builder grow on a path
	// that only a panic reaches.
	const panicSegmentEstimate = 64

	var sb strings.Builder

	sb.Grow(len(msg) + panicSegmentEstimate)

	writeFlattened(&sb, msg)
	writeOneLinePanicSite(&sb, stack)

	return sb.String()
}

// writeOneLinePanicSite appends one segment naming where a recovered panic was
// raised. No flattening needed: a file, line and function name resolved by the
// runtime can never contain \n, \r or \t.
//
// Symbolisation stays lazy — CallersFrames.Next resolves one frame and never
// touches the rest, however deep the stack. Only the clone StackTrace already
// made is paid for.
func writeOneLinePanicSite(sb *strings.Builder, stack []uintptr) {
	// No Grow here: oneLineEstimate does not size for this segment, but its
	// 64-byte leaf reserve plus size-class rounding already absorbs it.
	// Measured — BenchmarkRenderOneLinePanic is 440 B/op with or without one.
	resolved, _ := runtime.CallersFrames(stack).Next()

	sb.WriteString(OneLineSeparator)
	writeOneLineFrame(sb, Frame{
		Msg:      "panic",
		File:     resolved.File,
		Line:     resolved.Line,
		FuncName: resolved.Function,
	})
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
		// path.Base for the same reason as prettyEstimate: the frame renders
		// a base name, and reserving the full path tied the allocation size
		// to where the tree was checked out.
		size += framePrefix + len(f.Msg) + len(path.Base(f.File)) + len(f.FuncName)
	}

	if leaf != nil {
		size += len(OneLineSeparator) + leafReserve
	}

	return size
}
