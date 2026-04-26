package werr

import (
	"iter"
	"runtime"
)

// Frame is a resolved snapshot of a single werr wrap site.
type Frame struct {
	File     string // absolute path to the source file
	Line     int    // line number in File
	FuncName string // fully qualified function name, e.g. "pkg/sub.Func"
	Msg      string // message attached via [Wrapf]; empty for [Wrap]
}

// frameOf resolves an *Error's PC into a Frame. The runtime returns interned
// strings, so the resulting Frame is header-only — copying it does not
// allocate. Returns a zero-valued Frame (with Msg populated) if the PC could
// not be resolved.
func frameOf(err *Error) Frame {
	frame := Frame{Msg: err.msg} //nolint:exhaustruct
	if err.pc == 0 {
		return frame
	}

	rfn := runtime.FuncForPC(err.pc)
	if rfn == nil {
		return frame
	}

	frame.FuncName = rfn.Name()
	frame.File, frame.Line = rfn.FileLine(err.pc)

	return frame
}

// Walk iterates the werr-frames in err's chain, outermost to innermost,
// invoking fn for each. Iteration stops when fn returns false or when the
// next link is not a werr.Error.
//
// The return value depends on how iteration ended:
//
//   - Ran to completion: returns the leaf (the first non-werr error in
//     the chain, the "root cause" being wrapped).
//   - Stopped early (fn returned false): returns the error directly under
//     the frame fn rejected. That error may itself still be a *werr.Error.
//
// To always get the leaf, keep returning true (or use [StripAll]).
//
// Walk performs no heap allocations. If err is nil, returns nil.
//
// For a range-over-func variant when the return value isn't needed, see
// [Frames].
func Walk(err error, fn func(Frame) bool) error {
	for {
		we, ok := err.(*Error) //nolint:errorlint
		if !ok {
			return err
		}

		if !fn(frameOf(we)) {
			return we.err
		}

		err = we.err
	}
}

// Frames returns an [iter.Seq] over the werr-frames in err's chain,
// outermost to innermost. The sequence ends at the first non-werr error
// or when the consumer breaks out of the range loop:
//
//	for f := range werr.Frames(err) {
//	    log.Printf("wrap at %s:%d (%s)", f.File, f.Line, f.FuncName)
//	}
//
// Use [Walk] when you also need the leaf or the next-link error after
// early termination.
func Frames(err error) iter.Seq[Frame] {
	return func(yield func(Frame) bool) {
		for {
			we, ok := err.(*Error) //nolint:errorlint
			if !ok {
				return
			}

			if !yield(frameOf(we)) {
				return
			}

			err = we.err
		}
	}
}
