package werr

import (
	"errors"
	"testing"

	"github.com/gokern/panics"
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

// Rendering is the whole of what werr costs on the panic path. Each channel
// resolves a different amount of the stack: Pretty all of it, OneLine one
// frame, LogValue all of it as data.

//go:noinline
func raisePanicForBench() { panic("boom") }

func makePanicChain() error {
	return Wrapf(panics.Catch(raisePanicForBench), "delivering message")
}

func BenchmarkRenderPrettyPanic(b *testing.B) {
	err := makePanicChain()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = err.Error()
	}
}

func BenchmarkRenderOneLinePanic(b *testing.B) {
	err := makePanicChain()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = OneLine(err)
	}
}

func BenchmarkLogValuePanic(b *testing.B) {
	we, _ := AsWrap(makePanicChain())

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = we.LogValue()
	}
}
