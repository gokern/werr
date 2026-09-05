module github.com/gokern/werr/interop/sentry

go 1.26

require (
	github.com/getsentry/sentry-go v0.49.0
	github.com/gokern/panics v1.0.0
	github.com/stretchr/testify v1.12.1
)

require go.yaml.in/yaml/v3 v3.0.5 // indirect

require (
	github.com/gokern/werr/v2 v2.0.0
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
)

replace github.com/gokern/werr/v2 => ../..
