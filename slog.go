package werr

import "log/slog"

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

	// Inline the chain walk with a stack-allocated buffer. A closure-based
	// Walk would force the slice to grow incrementally with multiple heap
	// reallocations; the inline [16] keeps collection in one shot for
	// chains up to that depth. The slice itself still escapes once it is
	// passed to slog.Any below — that is unavoidable through the slog API.
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

	return slog.GroupValue(
		slog.String("msg", leafMsg),
		slog.Any("frames", frames),
	)
}
