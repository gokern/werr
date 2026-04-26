package werr

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/gokern/werr/internal/funcname"
	"github.com/gokern/werr/internal/pc"
)

// panicStackDepth bounds the frames captured for a panic stack.
const panicStackDepth = 64

// PanicToError converts a value recovered from a panic into a werr.Error
// carrying the panicking goroutine's stack:
//
//	defer func() {
//	    if r := recover(); r != nil {
//	        err = werr.PanicToError(r)
//	    }
//	}()
//
// PanicToError must be called from inside the deferred recover block.
// That is the only point at which the panic frames are still on the
// goroutine stack. Calling it after the defer returns captures only the
// surrounding function's call stack, not the panic site.
//
// The stack is rendered in the same " --- at <pkg>/<file>:<line> (<func>)"
// shape as [PrettyFormatter], so panic-recovered errors and werr-wrapped
// errors look uniform when printed.
//
// PanicToError(nil) returns nil. For a one-liner that handles recover()
// itself, see [Recover].
//
//go:noinline so the caller's PC sits in a real stack frame for asm pc.Caller.
func PanicToError(panicValue any) error {
	if panicValue == nil {
		return nil
	}

	caller := pc.Caller()

	return panicValueToError(
		panicValue,
		capturePanicStack(stackSkip),
		caller,
	)
}

// Recover is a deferred-function helper that catches a panic, converts it
// to a werr.Error, and stores it in *target:
//
//	func DoStuff() (err error) {
//	    defer werr.Recover(&err)
//	    panicProneCode()
//	    return nil
//	}
//
// Recover must appear directly in a defer statement. recover() only
// catches a panic when called from the deferred function itself, and
// Recover relies on being that function.
//
// If no panic occurs, *target is left unchanged. If target is nil, the
// panic is recovered but the resulting error is dropped — useful for the
// "don't crash" effect alone.
//
//go:noinline so the caller's PC sits in a real stack frame for asm pc.Caller.
func Recover(target *error) {
	panicValue := recover()
	if panicValue == nil {
		return
	}

	caller := pc.Caller()

	err := panicValueToError(
		panicValue,
		capturePanicStack(stackSkip),
		caller,
	)
	if target != nil {
		*target = err
	}
}

func panicValueToError(panicValue any, stack string, caller uintptr) error {
	switch v := panicValue.(type) {
	case error:
		// A typed-nil error (e.g. panic((*os.PathError)(nil)) or panic(MyErrFn(nil))
		// where MyErrFn is a func type satisfying error) shows up as a non-nil
		// interface holding a nil dynamic value. Wrapping it directly would
		// nil-deref later inside the formatter on leaf.Error(); render it via
		// %#v instead so the panic stays observable.
		//
		// Kinds listed below are the ones that satisfy reflect.Value.IsNil()
		// — calling IsNil on any other kind itself panics.
		rv := reflect.ValueOf(v)
		switch rv.Kind() { //nolint:exhaustive // only nilable kinds are relevant.
		case reflect.Pointer, reflect.Map, reflect.Chan, reflect.Func, reflect.Slice:
			if rv.IsNil() {
				return newError(fmt.Errorf("%#v", panicValue), stack, caller)
			}
		}

		return newError(v, stack, caller)
	case string:
		return newError(errors.New(v), stack, caller)
	default:
		return newError(fmt.Errorf("%#v", v), stack, caller)
	}
}

// capturePanicStack records the current goroutine's call stack and renders
// it in the same " --- at <pkg>/<basename>:<line> (<func>)" form used by
// [PrettyFormatter]. skip drops leading frames from the report (typically
// 3: runtime.Callers, capturePanicStack itself, and the public entry point).
//
// Compared with runtime/debug.Stack, this skips the "goroutine N [running]:"
// header and the runtime/debug internals, and avoids the []byte→string copy.
func capturePanicStack(skip int) string {
	var pcs [panicStackDepth]uintptr

	n := runtime.Callers(skip, pcs[:])
	if n == 0 {
		return "panic recovered\n"
	}

	frames := runtime.CallersFrames(pcs[:n])

	var sb strings.Builder

	sb.Grow(n*96 + len("panic recovered\n"))
	sb.WriteString("panic recovered\n")

	for {
		f, more := frames.Next()
		writePanicFrame(&sb, f)

		if !more {
			break
		}
	}

	return sb.String()
}

func writePanicFrame(sb *strings.Builder, frame runtime.Frame) {
	pkg, fn := funcname.Split(frame.Function)

	sb.WriteString(" --- at ")

	if pkg != "" {
		sb.WriteString(pkg)
		sb.WriteByte('/')
	}

	sb.WriteString(path.Base(frame.File))
	// Line is 0 for some inlined or runtime-internal frames; skip the
	// ":<line>" suffix in that case rather than emitting ":0".
	if frame.Line > 0 {
		sb.WriteByte(':')
		sb.WriteString(strconv.Itoa(frame.Line))
	}

	sb.WriteString(" (")
	sb.WriteString(fn)
	sb.WriteString(")\n")
}
