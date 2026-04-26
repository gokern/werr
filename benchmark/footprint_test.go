package benchmark

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"testing"

	errtrace "braces.dev/errtrace"
	emperror "emperror.dev/errors"
	cockroach "github.com/cockroachdb/errors"
	goerrors "github.com/go-errors/errors"
	"github.com/gokern/werr"
	"github.com/joomcode/errorx"
	mdobak "github.com/mdobak/go-xerrors"
	"github.com/palantir/stacktrace"
	pkgerrors "github.com/pkg/errors"
	"github.com/rotisserie/eris"
	werrold "github.com/safeblock-dev/werr"
	"github.com/samber/oops"
	"github.com/ztrue/tracerr"
	xerrors "golang.org/x/xerrors"
	tozd "gitlab.com/tozd/go/errors"
)

// Footprint measures the steady-state memory cost of one error: not the
// bytes allocated during creation (B/op, which is misleading for
// arena-pooled libs like werr) but the bytes still live on the heap once
// the error sits in a reachable slice.
//
// BenchmarkFootprint_<lib> calls runFootprint with the lib's idiomatic
// single-wrap factory. The helper holds every produced error in a
// pre-allocated slice across the bench loop, takes a HeapAlloc delta,
// and reports `live-B/err` as a custom metric via b.ReportMetric.
//
// Why not B/op: B/op is bytes-allocated-per-iteration. For non-arena
// libs that equals the live bytes, but werr and errtrace amortise slab
// allocs across iterations, so B/op understates the per-error retained
// size. live-B/err is what an operator actually sees in their RSS graph.
//
// The custom metric shows up as an extra column in `go test -bench`:
//
//	BenchmarkFootprint_werr-8   1000000   ... B/op   ...   57.0 live-B/err
//
// bench-chart reads it from RESULTS.txt alongside ns/op and B/op.

// runFootprint runs factory b.N times into a pre-allocated slice,
// measures the HeapAlloc delta, and reports two per-error metrics:
//
//   - header-B    unsafe.Sizeof of the concrete error struct (static)
//   - live-B/err  HeapAlloc delta per retained error (dynamic)
//
// Pre-allocating the slice keeps slice growth out of the live-B/err
// measurement; b.ResetTimer keeps the pre-grow phase out of ns/op. The
// throw-away factory() used for header sizing runs before runtime.GC,
// so its allocation is reclaimed before the measurement window opens.
func runFootprint(b *testing.B, factory func() error) {
	header := headerSize(factory()) // factory's allocation is reclaimed by the GC below

	holder := make([]error, 0, b.N)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		holder = append(holder, factory())
	}
	b.StopTimer()

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(holder)

	// Both ReportMetric calls go after ResetTimer; ResetTimer deletes
	// any user-reported metrics emitted before it.
	b.ReportMetric(float64(header), "header-B")

	if b.N == 0 || after.HeapAlloc <= before.HeapAlloc {
		return
	}
	live := float64(after.HeapAlloc-before.HeapAlloc) / float64(b.N)
	b.ReportMetric(live, "live-B/err")
}

// headerSize returns unsafe.Sizeof of the concrete error type (dereferenced
// once if it's a pointer).
func headerSize(err error) uintptr {
	v := reflect.ValueOf(err)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 0
		}
		v = v.Elem()
	}
	return v.Type().Size()
}

// Package-level leaf so the per-library bench wrappers stay one-liners
// and the benchmark name is the only library identifier.

//nolint:gochecknoglobals
var footprintLeaf = errors.New("leaf")

func BenchmarkFootprint_stdlib(b *testing.B) {
	runFootprint(b, func() error { return fmt.Errorf("%w", footprintLeaf) })
}
func BenchmarkFootprint_xerrors(b *testing.B) {
	runFootprint(b, func() error { return xerrors.Errorf(": %w", footprintLeaf) })
}
func BenchmarkFootprint_werr(b *testing.B) {
	runFootprint(b, func() error { return werr.Wrap(footprintLeaf) })
}
func BenchmarkFootprint_werrold(b *testing.B) {
	runFootprint(b, func() error { return werrold.Wrap(footprintLeaf) })
}
func BenchmarkFootprint_pkgerrors(b *testing.B) {
	runFootprint(b, func() error { return pkgerrors.WithStack(footprintLeaf) })
}
func BenchmarkFootprint_cockroachdb(b *testing.B) {
	runFootprint(b, func() error { return cockroach.WithStack(footprintLeaf) })
}
func BenchmarkFootprint_emperror(b *testing.B) {
	runFootprint(b, func() error { return emperror.WithStack(footprintLeaf) })
}
func BenchmarkFootprint_errorx(b *testing.B) {
	runFootprint(b, func() error { return errorx.EnsureStackTrace(footprintLeaf) })
}
func BenchmarkFootprint_goerrors(b *testing.B) {
	runFootprint(b, func() error { return goerrors.Wrap(footprintLeaf, 0) })
}
func BenchmarkFootprint_eris(b *testing.B) {
	runFootprint(b, func() error { return eris.Wrap(footprintLeaf, "") })
}
func BenchmarkFootprint_palantir(b *testing.B) {
	runFootprint(b, func() error { return stacktrace.Propagate(footprintLeaf, "") })
}
func BenchmarkFootprint_oops(b *testing.B) {
	runFootprint(b, func() error { return oops.Wrapf(footprintLeaf, "") })
}
func BenchmarkFootprint_tozd(b *testing.B) {
	runFootprint(b, func() error { return tozd.WithStack(footprintLeaf) })
}
func BenchmarkFootprint_errtrace(b *testing.B) {
	runFootprint(b, func() error { return errtrace.Wrap(footprintLeaf) })
}
func BenchmarkFootprint_mdobak(b *testing.B) {
	runFootprint(b, func() error { return mdobak.WithStackTrace(footprintLeaf, 0) })
}
func BenchmarkFootprint_tracerr(b *testing.B) {
	runFootprint(b, func() error { return tracerr.Wrap(footprintLeaf) })
}

