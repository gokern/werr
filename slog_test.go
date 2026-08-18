package werr_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/gokern/panics"
	"github.com/stretchr/testify/require"

	"github.com/gokern/werr/v2"
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
		// PanicFrames is a pointer so the tests can tell an absent key from
		// an empty array — that distinction is the contract.
		PanicFrames *[]struct {
			Func string `json:"func"`
			File string `json:"file"`
			Line int    `json:"line"`
		} `json:"panicFrames"`
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

//go:noinline
func raisePanicForSlog() { panic("boom") }

// panics ships no slog integration of its own and hands that to werr in its
// README. These tests are that hand-off.
func TestError_LogValue_panicFrames(t *testing.T) {
	t.Parallel()

	t.Run("absent when there is no panic", func(t *testing.T) {
		t.Parallel()

		got := decodeSlogErr(t, werr.Wrapf(errors.New("leaf"), "ctx"))
		require.Nil(t, got.Err.PanicFrames,
			"an ordinary error must not carry the key at all, not even empty")
	})

	t.Run("present with the panic site first", func(t *testing.T) {
		t.Parallel()

		got := decodeSlogErr(t, werr.Wrapf(panics.Catch(raisePanicForSlog), "delivering"))

		require.NotNil(t, got.Err.PanicFrames, "the key is the signal a dashboard filters on")
		require.NotEmpty(t, *got.Err.PanicFrames)

		frames := *got.Err.PanicFrames
		require.Contains(t, frames[0].Func, "raisePanicForSlog",
			"innermost first puts the panic site at index 0")
		require.NotEmpty(t, frames[0].File, "file must be the full path, as for wrap frames")
		require.Positive(t, frames[0].Line)

		require.Equal(t, "panic: boom", got.Err.Msg)
		require.Len(t, got.Err.Frames, 1, "wrap frames stay separate from panic frames")
	})

	t.Run("found when the chain buried the panic", func(t *testing.T) {
		t.Parallel()

		leaf := fmt.Errorf("task %q: %w", "sync", panics.Catch(raisePanicForSlog))

		got := decodeSlogErr(t, werr.Wrapf(leaf, "delivering"))
		require.NotNil(t, got.Err.PanicFrames,
			"panics.As must reach a panic behind fmt.Errorf, the idiom panics recommends")
	})
}
