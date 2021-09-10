GO=go

all: querylist inCountryLookup parseInCountryLookup resolverlist probegenerator whiteboard parsewhiteboard whiteboardresults v4vsv6

querylist: cmd/querylist/main.go
	cd cmd/querylist/ && $(GO) build -o querylist main.go && mv querylist ../../

inCountryLookup: cmd/inCountryLookup/main.go cmd/inCountryLookup/go.mod cmd/inCountryLookup/go.sum
	cd cmd/inCountryLookup/ && $(GO) build -o inCountryLookup main.go && mv inCountryLookup ../../

parseInCountryLookup: cmd/parseInCountryLookup/main.go cmd/parseInCountryLookup/go.mod cmd/parseInCountryLookup/go.sum
	cd cmd/parseInCountryLookup && $(GO) build -o parseInCountryLookup main.go && mv parseInCountryLookup ../../

resolverlist: cmd/resolverlist/main.go cmd/resolverlist/go.mod cmd/resolverlist/go.sum cmd/resolverlist/unique_asn.py
	cd cmd/resolverlist && $(GO) build -o resolverlist main.go && mv resolverlist ../../ && cp unique_asn.py ../../

probegenerator: cmd/probegenerator/main.go
	cd cmd/probegenerator && $(GO) build -o probegenerator main.go && mv probegenerator ../../

whiteboard: cmd/whiteboard/main.go cmd/whiteboard/go.mod cmd/whiteboard/go.sum
	cd cmd/whiteboard && $(GO) build -o whiteboard main.go && mv whiteboard ../../

parsewhiteboard: cmd/parseWhiteboard/parse_whiteboard_experiment.py
	cp cmd/parseWhiteboard/parse_whiteboard_experiment.py .

whiteboardresults: cmd/whiteboardresults/main.go cmd/whiteboardresults/go.mod cmd/whiteboardresults/go.sum
	cd cmd/whiteboardresults && $(GO) build -o whiteboardresults main.go && mv whiteboardresults ../../

v4vsv6: cmd/v4vsv6/main.go cmd/v4vsv6/go.mod
	cd cmd/v4vsv6 && $(GO) build -o v4vsv6 main.go && mv v4vsv6 ../../

.PHONY: clean all

clean:
	rm -f querylist inCountryLookup parseInCountryLookup resolverlist whiteboard whiteboardresults v4vsv6 unique_asn.py parse_whiteboard_experiment.py
