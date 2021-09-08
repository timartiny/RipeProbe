GO=go

all: querylist inCountryLookup parseresults probegenerator whiteboard whiteboardresults v4vsv6

querylist: cmd/querylist/main.go
	cd cmd/querylist/ && $(GO) build -o querylist main.go && mv querylist ../../

inCountryLookup: cmd/inCountryLookup/main.go cmd/inCountryLookup/go.mod cmd/inCountryLookup/go.sum
	cd cmd/inCountryLookup/ && $(GO) build -o inCountryLookup main.go && mv inCountryLookup ../../

parseresults: cmd/parseresults/main.go cmd/parseresults/go.mod cmd/parseresults/go.sum
	cd cmd/parseresults && $(GO) build -o parseresults main.go && mv parseresults ../../

probegenerator: cmd/probegenerator/main.go
	cd cmd/probegenerator && $(GO) build -o probegenerator main.go && mv probegenerator ../../

whiteboard: cmd/whiteboard/main.go cmd/whiteboard/go.mod cmd/whiteboard/go.sum
	cd cmd/whiteboard && $(GO) build -o whiteboard main.go && mv whiteboard ../../

whiteboardresults: cmd/whiteboardresults/main.go cmd/whiteboardresults/go.mod cmd/whiteboardresults/go.sum
	cd cmd/whiteboardresults && $(GO) build -o whiteboardresults main.go && mv whiteboardresults ../../

v4vsv6: cmd/v4vsv6/main.go cmd/v4vsv6/go.mod
	cd cmd/v4vsv6 && $(GO) build -o v4vsv6 main.go && mv v4vsv6 ../../

.PHONY: clean all

clean:
	rm -f querylist inCountryLookup parseresults whiteboard whiteboardresults v4vsv6
