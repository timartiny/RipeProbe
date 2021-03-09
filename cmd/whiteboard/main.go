package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"time"

	atlas "github.com/keltia/ripe-atlas"
)

var dataPrefix string
var infoLogger *log.Logger
var errorLogger *log.Logger

func getProbes() []atlas.Probe {
	client, err := atlas.NewClient(atlas.Config{})
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}
	opts := make(map[string]string)
	opts["status"] = "1"
	probes, err := client.GetProbes(opts)
	if err != nil {
		errorLogger.Fatalf("Error getting probes, err: %v\n", err)
	}
	return probes
}

func getNProbesNotCountry(size int, excludeCountry string) []atlas.Probe {
	var ret []atlas.Probe
	infoLogger.Printf("Grabbing all RIPE Atlas probes")
	allProbes := getProbes()
	numProbes := len(allProbes)
	s := rand.NewSource(time.Now().Unix())
	rg := rand.New(s)

	infoLogger.Printf(
		"Filtering down to %d probes not from %s\n", size, excludeCountry,
	)
	for len(ret) < size {
		rInd := rg.Intn(numProbes)
		if allProbes[rInd].CountryCode != excludeCountry {
			ret = append(ret, allProbes[rInd])
		}
	}

	return ret
}

func main() {
	dataPrefix = "../../data"
	numProbes := flag.Int("n", 0, "Number of probes to grab")
	countryCode := flag.String("c", "", "Country code to exclude from probes")
	flag.Parse()
	infoLogger = log.New(
		os.Stderr,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile,
	)
	errorLogger = log.New(
		os.Stderr,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile,
	)

	nProbes := getNProbesNotCountry(*numProbes, *countryCode)
	infoLogger.Printf("probes ids: [")
	for _, probe := range nProbes {
		infoLogger.Printf("\t%d", probe.ID)
	}
	infoLogger.Printf("]\n")
}
