module github.com/gokern/werr/benchmark

go 1.26

require (
	braces.dev/errtrace v0.4.0
	emperror.dev/errors v0.8.1
	github.com/cockroachdb/errors v1.13.0
	github.com/go-errors/errors v1.5.1
	github.com/go-playground/errors/v5 v5.4.0
	github.com/gokern/werr v0.0.0-00010101000000-000000000000
	github.com/joomcode/errorx v1.2.0
	github.com/mdobak/go-xerrors v1.0.0
	github.com/palantir/stacktrace v0.0.0-20161112013806-78658fd2d177
	github.com/pkg/errors v0.9.1
	github.com/rotisserie/eris v0.5.4
	github.com/safeblock-dev/werr v0.2.1
	github.com/samber/oops v1.22.0
	github.com/ztrue/tracerr v0.4.0
	gitlab.com/tozd/go/errors v0.11.1
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cockroachdb/logtags v0.0.0-20241215232642-bb51bb14a506 // indirect
	github.com/cockroachdb/redact v1.1.8 // indirect
	github.com/getsentry/sentry-go v0.46.0 // indirect
	github.com/go-playground/pkg/v5 v5.31.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/samber/lo v1.53.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
)

replace github.com/gokern/werr => ../

// sentry-go is pinned to v0.33.0 because cockroachdb/errors v1.12.0 (latest)
// imports its `report` subpackage transitively, and that subpackage uses
// Event.Extra which sentry-go removed in v0.34.0. Until cockroachdb/errors
// updates upstream, newer sentry-go fails to compile cockroachdb/errors/report.
// The interop/sentry module stays on the real latest (v0.46.0+) — that is the
// canonical sentry-go integration test for werr; this pin only affects the
// benchmark module and never werr itself.
