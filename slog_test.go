package werr_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gokern/werr"
)

// logRecord mirrors the JSON shape emitted when a werr.Error is logged
// under an "err" attribute via slog.JSONHandler.
type logRecord struct {
	Err struct {
		Msg    string `json:"msg"`
		Frames []struct {
			Func string `json:"func"`
			File string `json:"file"`
			Line int    `json:"line"`
			Msg  string `json:"msg"`
		} `json:"frames"`
	} `json:"err"`
}

func decodeSlogErr(t *testing.T, err error) logRecord {
	t.Helper()

	var buf bytes.Buffer

	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		// Drop time/level/msg only at the root. ReplaceAttr fires for every
		// attribute at every nesting level, and the "msg" inside the "err"
		// group must be preserved.
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 &&
				(a.Key == slog.TimeKey || a.Key == slog.LevelKey || a.Key == slog.MessageKey) {
				return slog.Attr{}
			}

			return a
		},
	})
	slog.New(h).Error("op failed", "err", err)

	var out logRecord

	require.NoError(t, json.Unmarshal(buf.Bytes(), &out), "raw output: %s", buf.String())

	return out
}

func TestError_LogValue(t *testing.T) {
	t.Parallel()

	t.Run("renders leaf msg and one frame for single Wrap", func(t *testing.T) {
		t.Parallel()

		err := werr.Wrap(errors.New("leaf"))
		got := decodeSlogErr(t, err)

		require.Equal(t, "leaf", got.Err.Msg)
		require.Len(t, got.Err.Frames, 1)
		require.Contains(t, got.Err.Frames[0].Func, "TestError_LogValue")
		require.Empty(t, got.Err.Frames[0].Msg, "plain Wrap leaves frame Msg empty")
		require.Positive(t, got.Err.Frames[0].Line)
	})

	t.Run("includes Wrapf message in the frame", func(t *testing.T) {
		t.Parallel()

		err := werr.Wrapf(errors.New("leaf"), "loading config")
		got := decodeSlogErr(t, err)

		require.Equal(t, "leaf", got.Err.Msg)
		require.Len(t, got.Err.Frames, 1)
		require.Equal(t, "loading config", got.Err.Frames[0].Msg)
	})

	t.Run("frames go outermost to innermost", func(t *testing.T) {
		t.Parallel()

		err := werr.Wrapf(werr.Wrap(werr.Wrapf(errors.New("leaf"), "load")), "register")
		got := decodeSlogErr(t, err)

		require.Equal(t, "leaf", got.Err.Msg)
		require.Len(t, got.Err.Frames, 3)
		require.Equal(t, "register", got.Err.Frames[0].Msg)
		require.Empty(t, got.Err.Frames[1].Msg)
		require.Equal(t, "load", got.Err.Frames[2].Msg)
	})

	t.Run("preserves full file path", func(t *testing.T) {
		t.Parallel()

		err := werr.Wrap(errors.New("leaf"))
		got := decodeSlogErr(t, err)

		require.Len(t, got.Err.Frames, 1)
		require.Contains(t, got.Err.Frames[0].File, "/")
		require.Contains(t, got.Err.Frames[0].File, "slog_test.go")
	})

	t.Run("frame shape is fixed even when msg is empty", func(t *testing.T) {
		t.Parallel()

		err := werr.Wrap(errors.New("leaf"))
		got := decodeSlogErr(t, err)

		// All four fields must appear in the JSON even when Msg is empty.
		raw, err2 := json.Marshal(got.Err.Frames[0])
		require.NoError(t, err2)
		require.Contains(t, string(raw), `"msg":""`)
	})

	t.Run("chain deeper than the inline buffer renders correctly", func(t *testing.T) {
		t.Parallel()

		// LogValue collects frames into a stack-allocated [16]slogFrame
		// buffer. Chains deeper than 16 force the slice to grow on the
		// heap; the resulting JSON must still carry every frame in order
		// without truncation or duplication.
		const depth = 32

		var top error

		top = errors.New("leaf")
		for range depth {
			top = werr.Wrap(top)
		}

		got := decodeSlogErr(t, top)

		require.Equal(t, "leaf", got.Err.Msg)
		require.Len(t, got.Err.Frames, depth, "every werr layer must appear as a frame")
	})
}
