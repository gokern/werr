package werr_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/gokern/werr"
)

// Error implements [slog.LogValuer], so the standard logger emits werr
// chains as nested JSON groups instead of multi-line text fields. This
// makes individual frames filterable in log aggregators.
func ExampleError_LogValue() {
	var buf bytes.Buffer

	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		// Strip non-deterministic top-level fields (time, level, msg).
		// The check on len(groups) keeps the leaf "msg" inside the err
		// group untouched.
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 {
				switch attr.Key {
				case slog.TimeKey, slog.LevelKey, slog.MessageKey:
					return slog.Attr{}
				}
			}

			return attr
		},
	})

	logger := slog.New(h)
	logger.Error("op failed", "err", werr.Wrapf(io.EOF, "loading"))

	// Verify the structured shape rather than the full JSON, since file
	// paths, line numbers, and function names depend on the runtime
	// location of this very example.
	var raw map[string]any

	_ = json.Unmarshal(buf.Bytes(), &raw)
	errGroup := raw["err"].(map[string]any)
	frame := errGroup["frames"].([]any)[0].(map[string]any)

	fmt.Println("leaf:", errGroup["msg"])
	fmt.Println("frames:", len(errGroup["frames"].([]any)))
	fmt.Println("frame msg:", frame["msg"])
	// frame["func"], frame["file"], frame["line"] are populated too, but
	// not asserted on because their values are runtime-dependent.

	// Output:
	// leaf: EOF
	// frames: 1
	// frame msg: loading
}
