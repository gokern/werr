package werr

import (
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/gokern/werr/v2/internal/funcname"
)

// PrettyFormatter renders the chain as a multi-line, Java-exception-style
// stack trace. Frame format is "<pkg>/<basename>:<line> (<func>)" where
// pkg is the full import path from runtime.FuncForPC and basename is
// path.Base of the source file (full file path is kept for slog only):
//
//	{outer message}
//	 --- at github.com/foo/bar/file.go:42 (FuncName)
//	 --- at github.com/foo/bar/other.go:84 (OtherFunc)
//	Caused by: {next message}
//	 --- at github.com/foo/bar/inner.go:11 (Inner)
//	Caused by: {leaf error text}
//
// Frames with empty Msg attach to the current heading without producing a
// "Caused by:" line. When the outermost frame has no Msg, the leaf becomes
// the heading and the trailing "Caused by:" is suppressed.
//
// When the leaf carries a recovered panic, its frames are appended in the
// same " --- at" shape after the "Caused by:" line, so a panic shows where it
// was raised and not only where it was caught. This holds with no wrap frames
// at all: a panic returned without a werr layer above it still renders its
// site, because panics ships no formatter and hands rendering here.
//
// Install as the global formatter via [SetPrettyFormatter], or call [Pretty]
// for one-off rendering. PrettyFormatter itself is exposed for composition:
//
//	werr.SetFormatter(func(frames []werr.Frame, leaf error) string {
//	    return "[APP] " + werr.PrettyFormatter(frames, leaf)
//	})
//
//nolint:cyclop // tight inline rendering with conservative branching keeps allocations down.
func PrettyFormatter(frames []Frame, leaf error) string {
	if len(frames) == 0 {
		return prettyLeafOnly(leaf)
	}

	leafMsg := ""
	if leaf != nil {
		leafMsg = leaf.Error()
	}

	headingFromLeaf := frames[0].Msg == ""
	heading := leafMsg

	if !headingFromLeaf {
		heading = frames[0].Msg
	}

	var sb strings.Builder

	sb.Grow(prettyEstimate(frames, heading, leafMsg, headingFromLeaf))

	// Skip empty heading to avoid a leading bare newline (e.g. errors.New("")).
	if heading != "" {
		sb.WriteString(heading)
		sb.WriteByte('\n')
	}

	currentMsg := frames[0].Msg
	for _, frame := range frames {
		if frame.Msg != "" && frame.Msg != currentMsg {
			sb.WriteString("Caused by: ")
			sb.WriteString(frame.Msg)
			sb.WriteByte('\n')

			currentMsg = frame.Msg
		}

		writePrettyFrame(&sb, frame)
	}

	leafTail := !headingFromLeaf && leafMsg != ""
	if leafTail {
		sb.WriteString("Caused by: ")
		sb.WriteString(leafMsg)
	}

	if stack := panicStack(leaf); stack != nil {
		// Wrap frames end in a newline, the "Caused by:" tail does not.
		// Emitting one unconditionally would put a blank line between the
		// wrap frames and the panic frames, which are meant to read as one
		// list.
		if leafTail {
			sb.WriteByte('\n')
		}

		writePanicFrames(&sb, stack)
	}

	return sb.String()
}

// prettyLeafOnly renders a leaf that has no werr layer above it. Ordinarily
// that is the leaf's own text and nothing else — the allocation-free answer
// this branch has always given.
//
// A recovered panic is the exception. panics has no formatters of its own and
// assigns rendering to werr, so a panic that reaches here unwrapped has
// nobody else to render it, and Pretty(p) must not hide a stack that
// Pretty(Wrap(p)) shows. The panics.As lookup costs one failed assertion on
// every ordinary error; the Builder is only raised once a panic is found.
func prettyLeafOnly(leaf error) string {
	if leaf == nil {
		return ""
	}

	msg := leaf.Error()

	stack := panicStack(leaf)
	if stack == nil {
		return msg
	}

	var sb strings.Builder

	sb.Grow(len(msg) + 1 + len(stack)*panicFrameEstimate)

	// Skip an empty heading for the same reason PrettyFormatter does: it
	// would open the output with a bare newline (errors.New("")).
	if msg != "" {
		sb.WriteString(msg)
		sb.WriteByte('\n')
	}

	writePanicFrames(&sb, stack)

	return sb.String()
}

// panicFrameEstimate is the Builder reserve per resolved panic frame: the
// " --- at " decoration, a package-qualified function name, a base file name
// and a line number.
const panicFrameEstimate = 96

// writePanicFrames appends the frames of a recovered panic, in the same shape
// as wrap frames. Otherwise pretty output would name the panic without saying
// where it happened.
//
// A chain joining several panics renders the first one [panics.As] reaches,
// which is what As is documented to do. Code that needs all of them walks
// Unwrap() []error rather than reading a formatted string.
func writePanicFrames(sb *strings.Builder, stack []uintptr) {
	// The caller's estimate sized the Builder before the panic was known
	// about, and the frame text does not exist until CallersFrames resolves
	// it below. Grow here or a deep panic walks the Builder through a
	// realloc cascade. No-op when the caller already reserved the room.
	sb.Grow(len(stack) * panicFrameEstimate)

	frames := runtime.CallersFrames(stack)

	for {
		resolved, more := frames.Next()

		writePrettyFrame(sb, Frame{ //nolint:exhaustruct // Msg has no meaning for a panic frame.
			File:     resolved.File,
			Line:     resolved.Line,
			FuncName: resolved.Function,
		})

		if !more {
			break
		}
	}
}

func writePrettyFrame(sb *strings.Builder, frame Frame) {
	pkg, fn := funcname.Split(frame.FuncName)

	sb.WriteString(" --- at ")

	if pkg != "" {
		sb.WriteString(pkg)
		sb.WriteByte('/')
	}

	// path.Base (forward-slash only) is correct here — runtime.FuncForPC
	// returns forward-slash paths on every platform, including Windows.
	// The same applies to format_oneline.go.
	sb.WriteString(path.Base(frame.File))

	if frame.Line > 0 {
		sb.WriteByte(':')
		sb.WriteString(strconv.Itoa(frame.Line))
	}

	sb.WriteString(" (")
	sb.WriteString(fn)
	sb.WriteString(")\n")
}

func prettyEstimate(frames []Frame, heading, leafMsg string, headingFromLeaf bool) int {
	const (
		causedByLen = len("Caused by: ") + 1
		framePrefix = len(" --- at ") + len(":") + len(" (") + len(")\n") + 8
	)

	size := len(heading) + 1
	for _, f := range frames {
		// path.Base, not len(f.File): the frame renders as a base name, so
		// reserving the whole path charges the estimate for the directory
		// the code was compiled in. That made the single render allocation
		// grow with the depth of the build tree — a 20-character-longer
		// checkout pushed a 15-frame chain from 2304 to 2688 B/op for
		// byte-identical output. path.Base returns a substring, so this is
		// a scan, not an allocation.
		size += framePrefix + len(f.FuncName) + len(path.Base(f.File)) + len(f.Msg)
	}

	size += len(frames) * causedByLen

	if !headingFromLeaf && leafMsg != "" {
		size += causedByLen + len(leafMsg)
	}

	return size
}
