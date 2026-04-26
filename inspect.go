package werr

import "errors"

// IsWrap reports whether err's chain contains a werr.Error.
func IsWrap(err error) bool {
	_, ok := errors.AsType[*Error](err)

	return ok
}

// AsWrap extracts the outermost werr.Error from err's chain. Equivalent to
// [errors.As] targeting `*werr.Error` with a more direct signature:
//
//	if w, ok := werr.AsWrap(err); ok {
//	    log.Println(w.FuncName(), w.File(), w.Line())
//	}
func AsWrap(err error) (*Error, bool) {
	return errors.AsType[*Error](err)
}

// Strip removes one werr layer, returning the error directly underneath.
// Returns err unchanged if it is not a werr.Error, or nil if err is nil.
//
// Use Strip when forwarding to code that does concrete-type assertions:
//
//	if myErr, ok := werr.Strip(err).(*MyError); ok { ... }
//
// Stripping is not needed for [errors.Is], [errors.As], or [fmt.Errorf]
// with %w; those traverse werr layers transparently.
func Strip(err error) error {
	if w, ok := err.(*Error); ok { //nolint:errorlint
		return w.err
	}

	return err
}

// StripAll removes every consecutive werr layer, stopping at the first
// non-werr wrapper (fmt.Errorf, custom types). For full traversal across
// every wrapper kind, loop on [errors.Unwrap] instead.
func StripAll(err error) error {
	for {
		we, ok := err.(*Error) //nolint:errorlint
		if !ok {
			return err
		}

		err = we.err
	}
}
