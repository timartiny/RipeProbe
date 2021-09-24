module github.com/timartiny/RipeProbe/cmd/runexperiment

replace github.com/timartiny/RipeProbe/RipeExperiment => ../../ripeexperiment

go 1.14

require (
	github.com/alexflint/go-arg v1.4.2
	github.com/keltia/proxy v0.9.5 // indirect
	github.com/keltia/ripe-atlas v0.0.0-20210506215806-13f0d38c56e7
	github.com/pkg/errors v0.9.1 // indirect
	github.com/timartiny/RipeProbe/probes v0.0.0-20210924185422-7b537c269738
	golang.org/x/net v0.0.0-20210924151903-3ad01bbaa167 // indirect
	golang.org/x/text v0.3.7 // indirect
)
