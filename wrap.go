package werr

import (
	"fmt"

	"github.com/gokern/werr/internal/pc"
)

// Wrap adds a werr layer to err, capturing the call site. Returns nil if
// err is nil, so `return werr.Wrap(err)` is safe unconditionally.
//
//go:noinline so the caller's PC sits in a real stack frame for asm pc.Caller.
func Wrap(err error) error {
	if err == nil {
		return nil
	}

	return newError(err, "", pc.Caller())
}

// Wrapf is [Wrap] with a printf-style context message. When called without
// args, format is treated as a literal string and fmt.Sprintf is skipped,
// so the common `werr.Wrapf(err, "context")` idiom doesn't pay for a
// format pass.
//
// Do not pass user-controlled input as the format string —
// `werr.Wrapf(err, userInput)` is the standard fmt format-string injection
// hazard. Use a literal format and let userInput be an arg:
// `werr.Wrapf(err, "input: %s", userInput)`.
//
//go:noinline so the caller's PC sits in a real stack frame for asm pc.Caller.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	return newError(err, msg, pc.Caller())
}

// Wrap2 forwards a (T, error) tuple while wrapping the error if non-nil.
// Lets the common Go return shape fit on one line:
//
//	return werr.Wrap2(io.ReadAll(r))
//
//go:noinline so the caller's PC sits in a real stack frame for asm pc.Caller.
func Wrap2[T any](val T, err error) (T, error) { //nolint: ireturn
	if err == nil {
		return val, nil
	}

	caller := pc.Caller()

	return val, newError(err, "", caller)
}

// Wrap3 forwards a (T1, T2, error) tuple while wrapping the error if
// non-nil. For shapes like image.Decode:
//
//	img, format, err := werr.Wrap3(image.Decode(r))
//
// werr stops at arity 3; Wrap4+ has no idiomatic Go counterpart.
//
//go:noinline so the caller's PC sits in a real stack frame for asm pc.Caller.
func Wrap3[T, T2 any](val T, val2 T2, err error) (T, T2, error) { //nolint: ireturn
	if err == nil {
		return val, val2, nil
	}

	caller := pc.Caller()

	return val, val2, newError(err, "", caller)
}
