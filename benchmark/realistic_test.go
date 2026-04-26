package benchmark

import (
	"errors"
	"fmt"
	"testing"

	errtrace "braces.dev/errtrace"
	emperror "emperror.dev/errors"
	cockroach "github.com/cockroachdb/errors"
	goerrors "github.com/go-errors/errors"
	goplay "github.com/go-playground/errors/v5"
	"github.com/gokern/werr"
	"github.com/joomcode/errorx"
	mdobak "github.com/mdobak/go-xerrors"
	"github.com/palantir/stacktrace"
	pkgerrors "github.com/pkg/errors"
	"github.com/rotisserie/eris"
	werrold "github.com/safeblock-dev/werr"
	"github.com/samber/oops"
	tozd "gitlab.com/tozd/go/errors"
	xerrors "golang.org/x/xerrors"
)

// Realistic benchmark model.
//
// Each iteration models one application "request" that produces an error,
// shaped like a layered Go service:
//
//	chainN → chainN-1 → ... → chain1 returns the leaf
//
// chainK (K > 1) is //go:noinline and ends with `return ops.wrap(err)` on
// its own line, so every wrap call resolves to a distinct runtime PC. A
// flat for-loop over `wrap` would give every call the same PC and would
// not stress per-frame metadata resolution.
//
// Chain depth rotates 5/6/7/8 across iterations (chainEntries below).
// Average wraps per iteration ≈ 5.5.
//
// Leaf rotates per iteration (tick % 6):
//
//	0     fresh leaf with context message   (1/6)
//	2, 4  fresh bare leaf                   (2/6)
//	1,3,5 imported third-party sentinel     (3/6)
//
// Three checks per iteration:
//
//   - errors.Is(err, sentinelImport)  hits 4/6 of iterations
//   - errors.Is(err, sentinelMissing) always misses, full chain traversal
//   - errors.As(err, &typedTarget)    always misses, full chain traversal
//
// 1% of iterations call err.Error() to model the production logging path.
// One number per library: wrap cost + traversal cost + occasional render.

// chain1 is the bottom of the stack: returns the leaf without wrapping.
//
//go:noinline
func chain1(_ libOps, leaf error) error { return leaf }

// chain2..chain8 each call the layer below and wrap once on the way up.
// Names are numbered so depth is obvious in stack traces and bench output.
// Each layer exists only to give its ops.wrap call a unique PC.

//go:noinline
func chain2(ops libOps, leaf error) error {
	if err := chain1(ops, leaf); err != nil {
		return ops.wrap(err)
	}
	return nil
}

//go:noinline
func chain3(ops libOps, leaf error) error {
	if err := chain2(ops, leaf); err != nil {
		return ops.wrap(err)
	}
	return nil
}

//go:noinline
func chain4(ops libOps, leaf error) error {
	if err := chain3(ops, leaf); err != nil {
		return ops.wrap(err)
	}
	return nil
}

//go:noinline
func chain5(ops libOps, leaf error) error {
	if err := chain4(ops, leaf); err != nil {
		return ops.wrap(err)
	}
	return nil
}

//go:noinline
func chain6(ops libOps, leaf error) error {
	if err := chain5(ops, leaf); err != nil {
		return ops.wrap(err)
	}
	return nil
}

//go:noinline
func chain7(ops libOps, leaf error) error {
	if err := chain6(ops, leaf); err != nil {
		return ops.wrap(err)
	}
	return nil
}

//go:noinline
func chain8(ops libOps, leaf error) error {
	if err := chain7(ops, leaf); err != nil {
		return ops.wrap(err)
	}
	return nil
}

// chainEntries[i] is the entry point for stack depth i+5 (so 5/6/7/8).
//
//nolint:gochecknoglobals
var chainEntries = [4]func(libOps, error) error{chain5, chain6, chain7, chain8}

// libOps captures the three idiomatic API shapes per library. Each library's
// package-level ops* var below fills in its own native APIs; libraries
// without a native shape fall back to stdlib equivalents (see opsErrtrace,
// opsMdobak).
type libOps struct {
	bareLeaf func() error          // tick%6 ∈ {2, 4} — fresh bare creation
	msgLeaf  func(id int) error    // tick%6 == 0 — wrap sentinelImport with a context message
	wrap     func(err error) error // every chain layer — message-less propagation
}

// runRealistic is the shared driver. Per-library bench wrappers below just
// pass their ops* var.
func runRealistic(b *testing.B, ops libOps) {
	b.ReportAllocs()
	tick := 0
	for b.Loop() {
		var leaf error
		switch tick % 6 {
		case 0:
			leaf = ops.msgLeaf(tick)
		case 2, 4:
			leaf = ops.bareLeaf()
		default: // 1, 3, 5
			leaf = sentinelImport
		}

		err := chainEntries[tick%4](ops, leaf)

		boolSink = errors.Is(err, sentinelImport)
		boolSink = errors.Is(err, sentinelMissing)
		var t *typedTarget
		boolSink = errors.As(err, &t)

		// Render at ~1% rate. Modulus 101 is prime and coprime with 4 and
		// 6, so rendered iterations rotate through every (depth, leaf)
		// combination instead of landing in the same bucket like tick%100
		// would.
		if tick%101 == 0 {
			stringSink = err.Error()
		}
		tick++
		errSink = err
	}
}

// ---------------------------------------------------------------------------
// Per-library ops definitions.
//
// Each ops* uses the library's idiomatic public API. Libraries without an
// equivalent for one of the three shapes (errtrace, mdobak have no message
// API) fall back to stdlib so the chain still has a usable leaf.
//
// msgLeaf wraps sentinelImport whenever the library can preserve it
// through the chain, so errors.Is(err, sentinelImport) keeps working.
// ---------------------------------------------------------------------------

//nolint:gochecknoglobals
var (
	opsStdlib = libOps{
		bareLeaf: func() error { return errors.New("not found") },
		msgLeaf:  func(id int) error { return fmt.Errorf("user %d: %w", id, sentinelImport) },
		wrap:     func(err error) error { return fmt.Errorf("%w", err) },
	}
	opsXerrors = libOps{
		bareLeaf: func() error { return xerrors.New("not found") },
		msgLeaf:  func(id int) error { return xerrors.Errorf("user %d: %w", id, sentinelImport) },
		wrap:     func(err error) error { return xerrors.Errorf(": %w", err) },
	}

	opsWerr = libOps{
		bareLeaf: func() error { return werr.Wrap(errors.New("not found")) },
		msgLeaf:  func(id int) error { return werr.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return werr.Wrap(err) },
	}
	opsWerrold = libOps{
		bareLeaf: func() error { return errors.New("not found") },
		msgLeaf:  func(id int) error { return werrold.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return werrold.Wrap(err) },
	}

	opsPkgerrors = libOps{
		bareLeaf: func() error { return pkgerrors.New("not found") },
		msgLeaf:  func(id int) error { return pkgerrors.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return pkgerrors.WithStack(err) },
	}
	opsCockroachdb = libOps{
		bareLeaf: func() error { return cockroach.New("not found") },
		msgLeaf:  func(id int) error { return cockroach.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return cockroach.WithStack(err) },
	}
	opsEmperror = libOps{
		bareLeaf: func() error { return emperror.New("not found") },
		msgLeaf:  func(id int) error { return emperror.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return emperror.WithStack(err) },
	}

	// errorx: msgLeaf uses Decorate, not IllegalState.Wrap. errorx's own
	// docs say IllegalState.Wrap hides the underlying error from
	// errors.Is, while Decorate "leaves the semantics totally intact",
	// so sentinelImport stays reachable through Unwrap — which is what
	// the bench's Is-checks rely on.
	opsErrorx = libOps{
		bareLeaf: func() error { return errorx.IllegalState.New("not found") },
		msgLeaf:  func(id int) error { return errorx.Decorate(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return errorx.Decorate(err, "") },
	}
	opsGoerrors = libOps{
		bareLeaf: func() error { return goerrors.New("not found") },
		msgLeaf: func(id int) error {
			return goerrors.WrapPrefix(sentinelImport, fmt.Sprintf("user %d", id), 0)
		},
		wrap: func(err error) error { return goerrors.WrapPrefix(err, "", 0) },
	}
	opsEris = libOps{
		bareLeaf: func() error { return eris.New("not found") },
		msgLeaf:  func(id int) error { return eris.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return eris.Wrap(err, "") },
	}
	// palantir: Propagate is the only public wrap API and does not expose
	// Unwrap, so errors.Is can't reach sentinelImport through a palantir
	// chain — palantir users go through stacktrace.RootCause instead.
	// The bench measures palantir's actual behaviour (Is short-circuits
	// at the first Propagate) without trying to coerce it.
	opsPalantir = libOps{
		bareLeaf: func() error { return stacktrace.NewError("not found") },
		msgLeaf:  func(id int) error { return stacktrace.Propagate(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return stacktrace.Propagate(err, "") },
	}
	opsOops = libOps{
		bareLeaf: func() error { return oops.New("not found") },
		msgLeaf:  func(id int) error { return oops.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return oops.Wrapf(err, "") },
	}
	opsTozd = libOps{
		bareLeaf: func() error { return tozd.New("not found") },
		msgLeaf:  func(id int) error { return tozd.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return tozd.Wrap(err, "") },
	}
	opsGoplay = libOps{
		bareLeaf: func() error { return goplay.New("not found") },
		msgLeaf:  func(id int) error { return goplay.Wrapf(sentinelImport, "user %d", id) },
		wrap:     func(err error) error { return goplay.Wrap(err, "") },
	}

	opsErrtrace = libOps{
		bareLeaf: func() error { return errtrace.Wrap(errors.New("not found")) },
		msgLeaf: func(id int) error {
			return errtrace.Wrap(fmt.Errorf("user %d: %w", id, sentinelImport))
		},
		wrap: func(err error) error { return errtrace.Wrap(err) },
	}
	opsMdobak = libOps{
		bareLeaf: func() error { return mdobak.WithStackTrace(errors.New("not found"), 0) },
		msgLeaf: func(id int) error {
			return mdobak.WithStackTrace(fmt.Errorf("user %d: %w", id, sentinelImport), 0)
		},
		wrap: func(err error) error { return mdobak.WithStackTrace(err, 0) },
	}
)

// ---------------------------------------------------------------------------
// Bench function wrappers, one per library. The benchmark name
// (BenchmarkRealistic_<lib>) is the only library identifier, so each
// wrapper stays a one-liner.
// ---------------------------------------------------------------------------

func BenchmarkRealistic_stdlib(b *testing.B)      { runRealistic(b, opsStdlib) }
func BenchmarkRealistic_xerrors(b *testing.B)     { runRealistic(b, opsXerrors) }
func BenchmarkRealistic_werr(b *testing.B)        { runRealistic(b, opsWerr) }
func BenchmarkRealistic_werrold(b *testing.B)     { runRealistic(b, opsWerrold) }
func BenchmarkRealistic_pkgerrors(b *testing.B)   { runRealistic(b, opsPkgerrors) }
func BenchmarkRealistic_cockroachdb(b *testing.B) { runRealistic(b, opsCockroachdb) }
func BenchmarkRealistic_emperror(b *testing.B)    { runRealistic(b, opsEmperror) }
func BenchmarkRealistic_errorx(b *testing.B)      { runRealistic(b, opsErrorx) }
func BenchmarkRealistic_goerrors(b *testing.B)    { runRealistic(b, opsGoerrors) }
func BenchmarkRealistic_eris(b *testing.B)        { runRealistic(b, opsEris) }
func BenchmarkRealistic_palantir(b *testing.B)    { runRealistic(b, opsPalantir) }
func BenchmarkRealistic_oops(b *testing.B)        { runRealistic(b, opsOops) }
func BenchmarkRealistic_tozd(b *testing.B)        { runRealistic(b, opsTozd) }
func BenchmarkRealistic_goplay(b *testing.B)      { runRealistic(b, opsGoplay) }
func BenchmarkRealistic_errtrace(b *testing.B)    { runRealistic(b, opsErrtrace) }
func BenchmarkRealistic_mdobak(b *testing.B)      { runRealistic(b, opsMdobak) }
