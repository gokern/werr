# Changelog

Notable changes to `werr`. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 2.0.0 — 2026-08-18

Panic recovery leaves werr and moves to
[`github.com/gokern/panics`](https://github.com/gokern/panics). Containing a
panic and describing where an error travelled are two jobs, and werr was doing
both badly: it captured the stack eagerly, rendered it into the wrap message,
and printed the result in an order nobody would have chosen. werr keeps the
half it is good at, which is rendering.

> **The import path is now `github.com/gokern/werr/v2`.** Nothing upgrades into
> this release by accident: `go get -u` leaves a 1.x consumer on 1.x, and the
> move is a deliberate edit to go.mod and every import line.

### Module path

- **`github.com/gokern/werr` becomes `github.com/gokern/werr/v2`.** Two
  exported functions are gone (below), which is a major change under semantic
  import versioning whatever the size of the audience. Shipping it as a minor
  would have been a lie the tooling believes: a `go get -u` — or a grouped
  Dependabot minor-and-patch pull request — would have pulled a build-breaking
  release in as a routine bump. 1.0.0 stays on the proxy, correct and working,
  for anyone not ready to move.

  ```
  go get github.com/gokern/werr/v2
  ```

  Update the import path, and the package name is still `werr`. The
  `github.com/gokern/werr/interop/sentry` submodule keeps its own path and
  requires `v2` from its first release.

  `github.com/gokern/panics` is deliberately not doing the same thing and never
  will: `*panics.Panic` is a shared `errors.As` target, and a second major of it
  would split the dependency graph the moment two modules disagreed about which
  to import. It stays v1 and grows additively.

### Removed

- **`werr.Recover` and `werr.PanicToError` are gone. Use `panics.Catch` or
  `panics.CatchError` and wrap what comes back.** Both were thin wrappers
  around a job `panics` already did, and going through werr made the captured
  stack worse: `PanicToError` called outside an unwinding panic left
  `github.com/gokern/werr.PanicToError` sitting in the user's stack as the
  first frame, because the trim that removes it keys on `runtime.gopanic`
  being present.

  The new shape puts the wrap where a wrap belongs — on the way out, with the
  context of what was being attempted:

  ```go
  if p := panics.Catch(deliver); p != nil {
      return werr.Wrapf(p, "delivering message %d", id)
  }
  ```

### Changed

- **A recovered panic renders in an order a human can follow.** 1.0.0 built the
  stack into the wrap message, which pushed the wrap frame below the panic
  frames and left the panic value stranded at the bottom after a blank line:

  ```
  delivering message 42
   --- at werr_test/main_test.go:22 (Handler)
  Caused by: panic recovered
   --- at runtime/panic.go:860 (gopanic)
   --- at werr_test/main_test.go:11 (deliver)
   ...

   --- at werr_test/main_test.go:15 (Handler.func1.deferwrap1)
  Caused by: kaboom
  ```

  Now the panic is the leaf, and every channel renders it where it belongs:

  ```
  delivering message 42
   --- at werr_test/main_test.go:21 (Handler)
  Caused by: panic: kaboom
   --- at werr_test/main_test.go:16 (deliver)
   ...
  ```

  `runtime.gopanic` is gone from the frame list too — `panics` trims the stack
  to the site that raised it.
- **`OneLineFormatter` names the panic site in one segment instead of inlining
  the whole stack.** 1.0.0 flattened the rendered stack into the message, so
  one panic in a trivial two-frame chain produced a 462-character line with
  ` --- at ` repeated six times inside it. The segment is now
  `panic at deliver.go:16 (deliver)` and the rest of the stack stays out of the
  line, which is what a line-based aggregator wants.
- **`Error.LogValue` emits a `panicFrames` array when the chain carries a
  panic.** Three fields per frame — `func`, `file`, `line` — innermost first,
  so `panicFrames[0]` is where it blew up. The key is absent, not empty, when
  there is no panic: its presence is the signal to filter on, and an empty
  array would cost every ordinary log line.
- **All three channels find the panic with `panics.As` rather than matching the
  leaf by type.** This matters more than it sounds. A panic almost never
  arrives as the bare leaf: `taskgroup.Run` returns `errors.Join(...)` of them,
  and the idiom `panics` itself recommends is
  `fmt.Errorf("%w: %w", ErrSentinel, p)`. A type match rendered nothing for
  both.
- **A wrap is not required to render a panic.** `Pretty` and `OneLine` look
  for one in their no-frames branch too, so a `*panics.Panic` handed straight
  to either — bare, joined, or behind `fmt.Errorf` — prints its site. `panics`
  has no formatters and assigns rendering to werr; without this, an error that
  never passed through `Wrap` had nothing anywhere that would print its stack.
  Ordinary errors pay one failed `panics.As` on that branch and still render
  with zero allocations.
- **Containing a panic no longer pays for a stack nobody reads.** 1.0.0
  resolved and formatted every frame at recover time — 2 385 B/op and 8
  allocs/op on an eight-frame fixture — whether or not the error was ever
  printed. `panics.Catch` costs 2 allocs, and the frames are resolved when
  something renders them.

  This is a trade, and the losing side is code that renders the same error
  more than once. 1.0.0 built the string once and reused it; now every
  `Error()` re-resolves the symbols. Rendering a panic through `Pretty` is
  840 B/op and 5 allocs/op, through `OneLine` 440 B/op and 4 allocs/op
  (it resolves one frame), and through `LogValue` 1 512 B/op and 8 allocs/op.
  An error you log once and drop is strictly cheaper than before; one you
  format in a loop is not.
- **werr has a runtime dependency for the first time:
  `github.com/gokern/panics`.** That module has none of its own.

### Fixed

- **Every render allocates less, and the amount no longer depends on where the
  tree was checked out.** `prettyEstimate` and `oneLineEstimate` sized the
  `strings.Builder` with `len(frame.File)` — the absolute path — while the
  writers emit `path.Base(frame.File)`. The reservation therefore grew with the
  depth of the build directory and pushed the single render allocation into a
  larger size class for byte-identical output: a 15-frame chain cost 2304 B/op
  from one checkout and 2688 B/op from another twenty characters deeper.
  Reserving the base name instead takes that chain to 1792 B/op, a 5-frame
  chain from 768 to 640, and `OneLine` from 2048 to 1536 and 704 to 576. Renders
  stay at one allocation. Verified byte-identical across linux/amd64 and
  darwin/arm64 and across a 38-character difference in checkout path;
  `TestFormatEstimates_doNotScaleWithCheckoutDepth` is the gate.

### Documentation

- `Callers` states why it never splices a panic stack into its result: a
  recovered panic carries a complete goroutine stack down to `runtime.goexit`,
  and the wrap sites are frames from that same stack, so joining the two places
  code outside `goexit`. Use `panics.As(err).StackTrace()` for the panic's own
  frames — `sentry-go` finds them there by reflection with no help from werr.
- `Error.StackTrace` records why its signature is `[]uintptr` and not a named
  slice type, which is the kind of thing somebody eventually tries to "fix".
- The `_examples/demo` panic section and the README both show the
  `panics.Catch` shape.

## Migration

Move the import path first, in go.mod and in every file that imports werr. The
package name does not change, so the identifiers in your code stay as they are:

```go
// before
import "github.com/gokern/werr"

// after
import "github.com/gokern/werr/v2"
```

```sh
go get github.com/gokern/werr/v2
go mod tidy
```

Then replace the defer helper with a `Catch` around the code that can panic. The
named return goes away with it:

```go
// before
func Deliver(msg Message) (err error) {
    defer werr.Recover(&err)
    handler(msg)
    return nil
}

// after
func Deliver(msg Message) error {
    if p := panics.Catch(func() { handler(msg) }); p != nil {
        return werr.Wrapf(p, "delivering message %d", msg.ID)
    }
    return nil
}
```

If the code being guarded already returns an error, `CatchError` carries it
through and a panic reports as `errors.Is(err, panics.ErrPanic)`:

```go
func Deliver(msg Message) error {
    return werr.Wrap(panics.CatchError(func() error { return handler(msg) }))
}
```

`PanicToError` callers own the `recover()` site already, so they keep it and
swap the conversion:

```go
defer func() {
    if p := panics.Recover(recover()); p != nil {
        err = werr.Wrap(p)
    }
}()
```

Detecting a recovered panic changes from "did werr produce this" to
`panics.Is(err)`, which also matches a panic contained by any other package
that recovers through `panics`. Code that read the panic's location out of the
formatted string should read it from `panics.As(err).StackTrace()` instead;
`werr.Callers` reports wrap sites only, as it always has.

## 1.0.0 — 2026-04-26

Initial release.
