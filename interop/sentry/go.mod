module github.com/gokern/werr/interop/sentry

go 1.26

require (
	github.com/getsentry/sentry-go v0.48.0
	github.com/gokern/panics v1.0.0
	github.com/stretchr/testify v1.12.0
)

require (
	github.com/gokern/werr/v2 v2.0.0
	github.com/kr/text v0.2.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/gokern/werr/v2 => ../..
