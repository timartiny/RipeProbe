module github.com/timartiny/RipeProbe/cmd/parseresults

replace github.com/timartiny/RipeProbe/results => ../../results

replace github.com/timartiny/RipeProbe/RipeExperiment => ../../ripeexperiment

go 1.16

require (
	github.com/alexflint/go-arg v1.4.2
	github.com/google/gopacket v1.1.19
	github.com/timartiny/RipeProbe/RipeExperiment v0.0.0-00010101000000-000000000000
	github.com/timartiny/RipeProbe/results v0.0.0-00010101000000-000000000000
)
