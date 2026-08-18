package werr

import (
	"runtime"

	"github.com/gokern/werr/v2/internal/arena"
)

// Error wraps another error with a captured caller PC and an optional
// context message. File, line, and function name are resolved from the
// PC lazily, only when an accessor or formatter reads them; errors that
// are checked and discarded without printing pay nothing for resolution.
//
// Error is used as a pointer type via [Wrap], [Wrapf], [Wrap2], and [Wrap3].
// To extract one from a chain use [AsWrap], or pass `*werr.Error` to
// [errors.As].
type Error struct {
	pc  uintptr
	err error
	msg string
}

//nolint:gochecknoglobals,mnd
var _arena = arena.New[Error](1024)

func newError(err error, msg string, pc uintptr) error {
	e := _arena.Take()
	e.pc = pc
	e.err = err
	e.msg = msg

	return e
}

// Error renders the chain using the formatter installed via [SetFormatter]
// (default: [PrettyFormatter]). The leaf, meaning the first non-werr error
// in the chain, is passed to the formatter alongside the collected
// werr-frames.
func (e *Error) Error() string {
	kind := formatterKind(_kind.Load())
	if kind == kindCustom {
		return errorCustom(e)
	}

	var stack [16]Frame

	frames := stack[:0]

	cur := error(e)

	for {
		we, ok := cur.(*Error) //nolint:errorlint
		if !ok {
			break
		}

		frames = append(frames, frameOf(we))
		cur = we.err
	}

	if kind == kindPretty {
		return PrettyFormatter(frames, cur)
	}

	return OneLineFormatter(frames, cur)
}

// Unwrap returns the underlying error. Part of the Go error-wrapping
// protocol used by [errors.Is], [errors.As], and `fmt.Errorf("%w", ...)`.
func (e *Error) Unwrap() error {
	return e.err
}

// Message returns the context message attached via [Wrapf], or "" if the
// error was created with [Wrap].
func (e *Error) Message() string {
	return e.msg
}

// FuncName returns the fully qualified name of the function where the wrap
// occurred. Resolved on every call; for repeated reads, prefer the [Frame]
// produced by [Walk].
func (e *Error) FuncName() string {
	if e.pc == 0 {
		return ""
	}

	rfn := runtime.FuncForPC(e.pc)
	if rfn == nil {
		return ""
	}

	return rfn.Name()
}

// File returns the absolute source-file path of the wrap site. Resolved on
// every call.
func (e *Error) File() string {
	if e.pc == 0 {
		return ""
	}

	rfn := runtime.FuncForPC(e.pc)
	if rfn == nil {
		return ""
	}

	file, _ := rfn.FileLine(e.pc)

	return file
}

// Line returns the source line number of the wrap site.
func (e *Error) Line() int {
	if e.pc == 0 {
		return 0
	}

	rfn := runtime.FuncForPC(e.pc)
	if rfn == nil {
		return 0
	}

	_, line := rfn.FileLine(e.pc)

	return line
}

// PC returns the raw program counter captured at the wrap site, or 0 if
// capture failed. Useful when the caller wants to do its own symbol
// resolution (e.g. building Sentry stack frames).
func (e *Error) PC() uintptr {
	return e.pc
}

// StackTrace returns the wrap-site PCs of all werr frames in the chain
// rooted at e, in innermost-first order.
//
// sentry-go discovers this method by reflection: it looks up "StackTrace"
// by name on the outermost error. Calling sentry.CaptureException(err)
// picks up the werr stack with no glue code on the user side.
//
// The signature is `[]uintptr` and not a named slice type on purpose.
// Consumers match this method by name and read the reflect.Kind of the
// elements, so `[]uintptr` and pkg/errors' `[]Frame` both work; there is
// no interface to conform to. CLAUDE.md has the survey behind that.
//
// Iteration stops at the first non-werr link, just like [Walk] and
// [Callers]. Wrapping a werr error with fmt.Errorf("%w", werrErr) hides
// the chain: sentry sees the outer fmt error and never reaches this
// method.
//
// Equivalent to Callers(e); see [Callers] for nil and non-werr handling,
// the zero-PC skip, and the typed-nil guard that catches a typed-nil
// receiver here too.
func (e *Error) StackTrace() []uintptr {
	return Callers(e)
}

// errorCustom is the slow path for user-installed formatters. Splitting it
// out keeps the indirect FormatFn call out of [Error.Error] so the on-stack
// frame buffer stays on the stack on the fast path.
func errorCustom(we *Error) string {
	// Inline [16]Frame buffer collapses the append cap-grow cascade
	// (1→2→4→8→16) into a single heap allocation. The backing array still
	// escapes through the indirect FormatFn call below, since escape
	// analysis cannot see past the function-pointer load, so this is one
	// alloc instead of many, not zero-alloc.
	var stack [16]Frame

	frames := stack[:0]
	cur := error(we)

	for {
		we, ok := cur.(*Error) //nolint:errorlint
		if !ok {
			break
		}

		frames = append(frames, frameOf(we))
		cur = we.err
	}

	return (*_formatter.Load())(frames, cur)
}
