// Package werr is a small error-wrapping library that captures the caller's
// file, line, and function name at every wrap site so error chains read like
// a stack trace.
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
// # Panic recovery
//
//   - [Recover] is a deferred-function helper: `defer werr.Recover(&err)`.
//   - [PanicToError] is the explicit primitive when you need to call
//     recover() yourself.
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
