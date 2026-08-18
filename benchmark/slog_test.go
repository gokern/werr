package benchmark

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/gokern/werr/v2"
	"github.com/samber/oops"
)

// BenchmarkSlogJSON measures the cost of feeding a wrapped error to a
// slog.JSONHandler, the production path for services emitting structured
// JSON logs. The handler writes to io.Discard so the bench measures
// encoding cost, not I/O.
//
// werr and oops implement slog.LogValuer and emit a structured group;
// stdlib emits a plain string via Error(). This bench surfaces the cost
// each LogValuer adds on top of stdlib's encoder. Stack-capture libs
// without a LogValue method (mdobak/go-xerrors and friends) degrade to
// stdlib's path and are skipped here; they would just measure stdlib
// twice.

func newSlogJSON() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func BenchmarkSlogJSON_stdlib(b *testing.B) {
	logger := newSlogJSON()
	err := error(errLeaf)
	for range chainDepth {
		err = fmt.Errorf("%w", err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.LogAttrs(ctx, slog.LevelError, "request failed", slog.Any("err", err))
	}
}

func BenchmarkSlogJSON_werr(b *testing.B) {
	logger := newSlogJSON()
	err := error(errLeaf)
	for range chainDepth {
		err = werr.Wrap(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.LogAttrs(ctx, slog.LevelError, "request failed", slog.Any("err", err))
	}
}

func BenchmarkSlogJSON_oops(b *testing.B) {
	logger := newSlogJSON()
	err := error(errLeaf)
	for range chainDepth {
		err = oops.Wrapf(err, "")
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.LogAttrs(ctx, slog.LevelError, "request failed", slog.Any("err", err))
	}
}

