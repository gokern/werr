package werr

import (
	"path"
	"strconv"
	"strings"

	"github.com/gokern/werr/internal/funcname"
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
		if leaf == nil {
			return ""
		}

		return leaf.Error()
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

	if !headingFromLeaf && leafMsg != "" {
		sb.WriteString("Caused by: ")
		sb.WriteString(leafMsg)
	}

	return sb.String()
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
	// The same applies to format_oneline.go and panic.go.
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
		size += framePrefix + len(f.FuncName) + len(f.File) + len(f.Msg)
	}

	size += len(frames) * causedByLen

	if !headingFromLeaf && leafMsg != "" {
		size += causedByLen + len(leafMsg)
	}

	return size
}
