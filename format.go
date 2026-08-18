package werr

import (
	"sync/atomic"

	"github.com/gokern/panics"
)

// FormatFn renders a werr error chain to a string. It receives the
// werr-frames outermost to innermost plus the leaf (the first non-werr
// error in the chain). The slice is owned by the caller; copy it before
// retaining it past the call.
type FormatFn func(frames []Frame, leaf error) string

// formatterKind lets [Error.Error] dispatch to a built-in without an
// indirect call, which would otherwise force the on-stack frame buffer
// to escape.
type formatterKind int32

const (
	kindCustom formatterKind = iota
	kindPretty
	kindOneLine
)

//nolint:gochecknoglobals
var (
	_formatter atomic.Pointer[FormatFn]
	_kind      atomic.Int32
)

//nolint:gochecknoinits
func init() {
	var fn FormatFn = PrettyFormatter

	_formatter.Store(&fn)
	_kind.Store(int32(kindPretty))
}

// SetPrettyFormatter selects [PrettyFormatter] as the global formatter.
// Externally equivalent to `SetFormatter(PrettyFormatter)` but takes the
// fast direct-dispatch path inside [Error.Error].
//
// Process-global. Library code must not call it; use [Pretty] instead.
func SetPrettyFormatter() {
	// _formatter is intentionally not updated. The fast path in Error.Error
	// dispatches by _kind alone and never reads _formatter under kindPretty,
	// so any prior custom value sits dormant. Touching it here would require
	// a fresh heap allocation (FormatFn pointer), which is the cost
	// SetFormatter pays and SetPrettyFormatter exists to avoid.
	_kind.Store(int32(kindPretty))
}

// SetOneLineFormatter selects [OneLineFormatter] as the global formatter.
// Externally equivalent to `SetFormatter(OneLineFormatter)` but takes the
// fast direct-dispatch path inside [Error.Error].
//
// Process-global. Library code must not call it; use [OneLine] instead.
func SetOneLineFormatter() {
	// See SetPrettyFormatter for why _formatter is left untouched.
	_kind.Store(int32(kindOneLine))
}

// SetFormatter installs a custom [FormatFn] for [Error.Error]. The install
// is atomic and safe under concurrent reads. Passing nil is a no-op.
//
// For the built-ins, prefer [SetPrettyFormatter] and [SetOneLineFormatter]:
// they avoid the indirect call SetFormatter introduces, which costs one
// extra alloc per render.
//
// Process-global. Library code must not call it; use [Pretty] or [OneLine]
// for one-off rendering.
func SetFormatter(fn FormatFn) {
	if fn == nil {
		return
	}

	_formatter.Store(&fn)
	_kind.Store(int32(kindCustom))
}

// Pretty renders err with [PrettyFormatter] without touching the global
// formatter setting.
func Pretty(err error) string {
	if err == nil {
		return ""
	}

	var stack [16]Frame

	frames := stack[:0]
	cur := err

	for {
		we, ok := cur.(*Error) //nolint:errorlint
		if !ok {
			break
		}

		frames = append(frames, frameOf(we))
		cur = we.err
	}

	// Always go through PrettyFormatter, even for non-werr leaves: its
	// empty-frames branch is what renders a panic that arrived without a werr
	// layer above it. Returning cur.Error() here would make Pretty(p) and
	// Pretty(Wrap(p)) disagree on whether the panic site is visible.
	return PrettyFormatter(frames, cur)
}

// OneLine renders err with [OneLineFormatter] without touching the global
// formatter setting.
func OneLine(err error) string {
	if err == nil {
		return ""
	}

	var stack [16]Frame

	frames := stack[:0]
	cur := err

	for {
		we, ok := cur.(*Error) //nolint:errorlint
		if !ok {
			break
		}

		frames = append(frames, frameOf(we))
		cur = we.err
	}

	// Always go through OneLineFormatter, even for non-werr leaves: its
	// empty-frames branch flattens \n/\r/\t out of the leaf text, which
	// the single-line guarantee depends on.
	return OneLineFormatter(frames, cur)
}

// panicStack returns the stack of a recovered panic carried anywhere in err,
// or nil when there is none. Every panic lookup in werr goes through here.
//
// The search is [panics.As], not a type assertion on the leaf: a panic almost
// never arrives bare. taskgroup returns errors.Join of them, and the idiom
// panics itself recommends is fmt.Errorf("%w: %w", ErrSentinel, p). Matching
// by type rendered nothing for either.
//
// Returning nil rather than an empty slice keeps a caller from opening a
// section it has nothing to put in. StackTrace clones the retained stack, so
// callers pass the result along instead of asking twice.
func panicStack(err error) []uintptr {
	p, ok := panics.As(err)
	if !ok {
		return nil
	}

	stack := p.StackTrace()
	if len(stack) == 0 {
		return nil
	}

	return stack
}
