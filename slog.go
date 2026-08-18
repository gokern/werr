package werr

import (
	"log/slog"
	"runtime"
)

// LogValue implements [slog.LogValuer], producing a structured group for
// JSON log handlers:
//
//	{
//	    "msg":    "<leaf error text>",
//	    "frames": [
//	        {"func": "...", "file": "...", "line": 42, "msg": "..."},
//	        ...
//	    ]
//	}
//
// Used as `slog.Error("op failed", "err", err)`, the JSON output carries
// the leaf text and the werr-frames as separate fields, so log aggregators
// (Loki, ELK, Grafana) can filter by frame metadata instead of regex-matching
// a multi-line text blob.
//
// "frames" is ordered outermost to innermost. Every frame keeps all four
// fields, including empty "msg" for plain [Wrap] sites; the JSON shape
// is fixed so dashboards can rely on consistent field presence. "file"
// is the full path. Formatter helpers trim it for human-readable output,
// but structured logs keep full location info.
//
// # Recovered panics
//
// When the chain carries one, a "panicFrames" key appears alongside
// "frames", innermost first, so the panic site is panicFrames[0]:
//
//	{
//	    "msg":         "panic: kaboom",
//	    "frames":      [...],
//	    "panicFrames": [{"func": "...", "file": "...", "line": 16}]
//	}
//
// The two lists answer different questions: "frames" is where the error
// travelled, "panicFrames" is where the process nearly died.
//
// The key is absent, not empty, when there is no panic — the one break in the
// fixed-shape rule above. Presence of the key is itself the "this was a panic"
// signal a dashboard filters on, and an empty array would cost every ordinary
// log line for a case that is rare by construction.
func (e *Error) LogValue() slog.Value {
	// slogFrame is rendered to JSON via its struct tags. slog.Value would
	// not work here: []slog.Value is opaque to encoding/json (its fields
	// are unexported), and slog handlers serialise "any"-kind attributes
	// through json.Marshal.
	type slogFrame struct {
		Func string `json:"func"`
		File string `json:"file"`
		Line int    `json:"line"`
		Msg  string `json:"msg"`
	}

	// Inline the chain walk with a stack-sized buffer. The slice always
	// escapes when passed to slog.Any below (slog handlers serialise
	// "any"-kind attributes through json.Marshal), so the backing array
	// ends up on the heap regardless. The [16] cap saves the intermediate
	// cap-grow reallocations during the walk, nothing more.
	var stack [16]slogFrame

	frames := stack[:0]

	cur := error(e)

	for {
		we, ok := cur.(*Error) //nolint:errorlint
		if !ok {
			break
		}

		frame := frameOf(we)
		frames = append(frames, slogFrame{
			Func: frame.FuncName,
			File: frame.File,
			Line: frame.Line,
			Msg:  frame.Msg,
		})

		cur = we.err
	}

	leafMsg := ""
	if cur != nil {
		leafMsg = cur.Error()
	}

	// Two straight-line calls rather than one built slice. A slice sized for
	// the panic case costs the ordinary case 48 B/op: three slog.Attr is 120
	// bytes, which rounds into the 128-byte size class, where two is 80.
	// Ordinary errors are the overwhelming majority of what gets logged, so
	// they are the case that must not pay.
	if panicFrames := panicSlogFrames(cur); panicFrames != nil {
		return slog.GroupValue(
			slog.String("msg", leafMsg),
			slog.Any("frames", frames),
			slog.Any("panicFrames", panicFrames),
		)
	}

	return slog.GroupValue(
		slog.String("msg", leafMsg),
		slog.Any("frames", frames),
	)
}

// slogPanicFrame is the three-field shape of a panic frame in the log record.
// It has no "msg": a panic frame carries no wrap message, and emitting an
// always-empty field would invite a dashboard to filter on it.
type slogPanicFrame struct {
	Func string `json:"func"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// panicSlogFrames resolves the stack of a recovered panic carried by leaf.
// Returns nil when there is none, which is what keeps the key out of the
// record.
func panicSlogFrames(leaf error) []slogPanicFrame {
	stack := panicStack(leaf)
	if stack == nil {
		return nil
	}

	out := make([]slogPanicFrame, 0, len(stack))
	frames := runtime.CallersFrames(stack)

	for {
		resolved, more := frames.Next()

		out = append(out, slogPanicFrame{
			Func: resolved.Function,
			File: resolved.File,
			Line: resolved.Line,
		})

		if !more {
			break
		}
	}

	return out
}
