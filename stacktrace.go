package werr

// Callers returns the wrap-site PCs from err's chain in innermost-first
// order: index 0 is closest to the leaf, the last entry is the outermost
// wrap. The order matches runtime.Callers, so the result feeds straight
// into runtime.CallersFrames or any APM SDK that consumes []uintptr
// (Sentry, OpenTelemetry, Datadog).
//
// Like [Walk], iteration stops at the first non-werr link, so chains that
// cross fmt.Errorf or errors.Join lose everything past the boundary.
// Frames with pc == 0 are dropped, not zero-padded.
//
// Returns nil if err is nil or its outermost link is not a *Error.
// Allocates one slice, sized to the surviving frames.
//
// A typed-nil *Error anywhere in the chain (head or interior) ends the
// walk: we break before dereferencing. Real wrap chains never produce
// typed-nil interior links, but reflection callers like sentry-go can
// hand us a typed-nil receiver, and the same check covers both.
func Callers(err error) []uintptr {
	// Two-pass on purpose: count first to size the slice exactly, then
	// fill from the tail so outermost-first chain order flips into
	// innermost-first output. We tried single-pass + inline buffer +
	// reverse-copy; it benched ~5% slower because append's cap-check
	// inside the chain walk costs more than the direct indexed writes
	// used here, and arena locality means the "second traversal is
	// expensive" argument doesn't actually apply.
	n := 0

	cur := err
	for {
		we, ok := cur.(*Error) //nolint:errorlint
		if !ok || we == nil {
			break
		}

		if we.pc != 0 {
			n++
		}

		cur = we.err
	}

	if n == 0 {
		return nil
	}

	pcs := make([]uintptr, n)
	i := n - 1

	cur = err
	for {
		we, ok := cur.(*Error) //nolint:errorlint
		if !ok || we == nil {
			break
		}

		if we.pc != 0 {
			pcs[i] = we.pc
			i--
		}

		cur = we.err
	}

	return pcs
}
