module github.com/timartiny/RipeProbe/cmd/whiteboardresults

replace github.com/timartiny/RipeProbe/results => ../../results

go 1.16

require (
	github.com/google/gopacket v1.1.19
	github.com/keltia/ripe-atlas v0.0.0-20190416222805-da828cc7507d
	github.com/timartiny/RipeProbe/results v0.0.0-00010101000000-000000000000
)
