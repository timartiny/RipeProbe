module github.com/timartiny/RipeProbe/cmd/runexperiment

replace github.com/timartiny/RipeProbe/RipeExperiment => ../../ripeexperiment

go 1.14

require (
	github.com/keltia/ripe-atlas v0.0.0-20190416222805-da828cc7507d
	github.com/timartiny/RipeProbe/RipeExperiment v0.0.0-00010101000000-000000000000
	github.com/timartiny/RipeProbe/probes v0.0.0-20210924175406-fd02bee2b336 // indirect
	github.com/zmap/zflags v1.4.0-beta.1 // indirect
)
