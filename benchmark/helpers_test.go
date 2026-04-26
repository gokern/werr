// Package benchmark compares github.com/gokern/werr against other Go
// error-wrapping libraries under realistic and microbench scenarios.
//
// All comparison benchmarks live in this directory and share the helpers
// defined here. Library imports use stable aliases. The suffix on every
// BenchmarkRealistic_<lib> / BenchmarkFootprint_<lib> / BenchmarkSlogJSON_<lib>
// name maps 1:1 to one of:
//
//	stdlib       errors.New + fmt.Errorf("%w", ...)  (no third-party dep)
//	xerrors      golang.org/x/xerrors                (stdlib precursor)
//	werr         github.com/gokern/werr              (this repo)
//	werrold      github.com/safeblock-dev/werr       (predecessor)
//	pkgerrors    github.com/pkg/errors               (classic)
//	cockroachdb  github.com/cockroachdb/errors       (full stack + features)
//	emperror     emperror.dev/errors                 (pkg/errors-style)
//	errorx       github.com/joomcode/errorx          (typed namespaces)
//	goerrors     github.com/go-errors/errors         (full stack)
//	eris         github.com/rotisserie/eris          (modern, full stack)
//	palantir     github.com/palantir/stacktrace      (one frame per Propagate)
//	oops         github.com/samber/oops              (rich context)
//	tozd         gitlab.com/tozd/go/errors           (recent, "fast" claim)
//	goplay       github.com/go-playground/errors/v5  (stable, niche)
//	errtrace     braces.dev/errtrace                 (//line directive)
//	mdobak       github.com/mdobak/go-xerrors        (slog-first)
package benchmark

import "errors"

// Global escape sinks shared by every benchmark in this package. Assignments
// to these prevent the compiler from eliminating the work being measured.
//
//nolint:gochecknoglobals
var (
	errSink    error
	stringSink string
	boolSink   bool
)

// errLeaf is the leaf used by the slog and footprint scenarios. The realistic
// benchmark uses sentinelImport / sentinelMissing below so it can model both
// errors.Is hits and misses.
//
//nolint:gochecknoglobals
var errLeaf = errors.New("benchmark leaf error")

// chainDepth is the fixed depth used by the slog scenario. The realistic
// benchmark uses a rotating 5..8 depth instead (see realistic_test.go).
const chainDepth = 15

// sentinelImport models a leaf "imported" from a third-party package
// (e.g. sql.ErrNoRows, io.EOF). On half of the realistic iterations it is
// passed through as the chain's leaf, so errors.Is(err, sentinelImport)
// hits and walks the wrap chain to reach it.
//
//nolint:gochecknoglobals
var sentinelImport = errors.New("user not found")

// sentinelMissing is never inserted into any chain. The realistic
// benchmark calls errors.Is(err, sentinelMissing) once per iteration to
// force every library to traverse the full chain before failing to
// match. Production code does the same whenever it checks for an
// unrelated sentinel.
//
//nolint:gochecknoglobals
var sentinelMissing = errors.New("permission denied")

// typedTarget is the target of a never-matching errors.As call in the
// realistic benchmark. Like sentinelMissing it is never inserted into any
// chain, so As must walk the full chain and return false.
type typedTarget struct{ Code int }

func (t *typedTarget) Error() string { return "typed target" }
