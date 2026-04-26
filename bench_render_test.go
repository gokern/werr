package werr

import (
	"errors"
	"testing"
)

// Render-path benchmarks for werr internals. The cross-library suite
// lives under benchmark/.

func makeChain(depth int) error {
	err := errors.New("leaf: connection refused")
	for i := range depth {
		err = Wrapf(err, "step %d: dial backend", i)
	}

	return err
}

func BenchmarkRenderPretty5(b *testing.B) {
	err := makeChain(5)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = err.Error()
	}
}

func BenchmarkRenderOneLine5(b *testing.B) {
	err := makeChain(5)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = OneLine(err)
	}
}

func BenchmarkRenderPretty15(b *testing.B) {
	err := makeChain(15)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = err.Error()
	}
}

func BenchmarkRenderOneLine15(b *testing.B) {
	err := makeChain(15)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = OneLine(err)
	}
}

func BenchmarkCallers_chain8(b *testing.B) {
	err := makeChain(8)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = Callers(err)
	}
}
