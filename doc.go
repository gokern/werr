// Package werr is a small error-wrapping library. Each wrap stores one program
// counter, 8 bytes, and resolves it into file, line and function name only when
// the error is rendered. A chain of them reads like a stack trace of the points
// you chose to mark.
//
// # Wrapping
//
//   - [Wrap], [Wrapf], [Wrap2], [Wrap3] add a frame to an existing error.
//     [Wrap2] and [Wrap3] forward the (T, error) and (T1, T2, error) return
//     shapes in one expression.
//   - All four return nil when given nil, so `return werr.Wrap(err)` is safe
//     unconditionally.
//
// # Inspection
//
//   - [IsWrap] and [AsWrap] traverse the full chain, including past
//     fmt.Errorf("%w", ...).
//   - [Walk] iterates werr-frames outermost to innermost; [Frame] exposes the
//     metadata for each frame. [Frames] is a range-over-func variant for the
//     callsites that don't need [Walk]'s leaf return value.
//   - [Callers] returns wrap-site PCs innermost-first (opposite to [Walk]),
//     ready to feed into runtime.CallersFrames or APM SDKs.
//   - From a `*werr.Error` directly: [Error.Message], [Error.FuncName],
//     [Error.File], [Error.Line], [Error.PC].
//
// # Stripping werr layers
//
//   - [Strip] removes one werr layer.
//   - [StripAll] removes every consecutive werr layer, stopping at the first
//     non-werr wrapper.
//
// Stripping is only needed when forwarding to code that does concrete-type
// assertions; [errors.Is] and [errors.As] traverse werr layers transparently.
//
// # Recovered panics
//
// werr does not recover panics; containing one is a separate job, and
// [github.com/gokern/panics] does it:
//
//	if p := panics.Catch(deliver); p != nil {
//	    return werr.Wrapf(p, "delivering message %d", id)
//	}
//
// What werr does is render what comes back. [PrettyFormatter] appends the
// panic's own frames after the leaf, [OneLineFormatter] names the panic site
// in one further segment, and [Error.LogValue] emits them as "panicFrames".
// All three find the panic with panics.As, which reaches it through
// errors.Join and fmt.Errorf. The wrap is optional for rendering: [Pretty]
// and [OneLine] show the panic site on an error with no werr layer at all.
//
// [Callers] reports only werr's own wrap sites; reach the panic stack through
// panics.As(err).StackTrace(), which is also where sentry-go finds it by
// reflection.
//
// # Formatting
//
//   - [SetFormatter] installs a custom [FormatFn]. Process-global; library
//     code must not call it.
//   - [PrettyFormatter] (multi-line, default) and [OneLineFormatter]
//     (single-line, suitable for grep / Loki / ELK).
//   - [Pretty] and [OneLine] render directly without touching the global
//     formatter.
//
// # Structured logging
//
// [Error.LogValue] implements [log/slog.LogValuer], so slog handlers emit
// werr chains as structured groups instead of multi-line text blobs.
package werr
