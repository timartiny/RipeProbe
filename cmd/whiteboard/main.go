package main

import (
	"bufio"
	"flag"
	"fmt"
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

func writeProbesToFile(path string, probes []string) {
	file, err := os.Create(path)
	if err != nil {
		errorLogger.Fatalf("Error creating file: %s, %v\n", path, err)
	}

	for _, id := range probes {
		file.WriteString(id)
	}
}

func getNIDs(num int, path string) []string {
	var ret []string
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", path, err)
	}

	scanner := bufio.NewScanner(file)
	size := 0
	for scanner.Scan() {
		ret = append(ret, scanner.Text())
		size++
		if size >= num {
			break
		}
	}

	infoLogger.Printf("Returning %d probe ids\n", size)

	return ret
}

func main() {
	dataPrefix = "../../data"
	numProbes := flag.Int("n", 0, "Number of probes to grab")
	countryCode := flag.String("c", "", "Country code to exclude from probes")
	probesPath := flag.String("p", "", "Path to file containing list of probe Ids")
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
	var nProbeIDs []string
	if len(*probesPath) > 0 {
		nProbeIDs = getNIDs(
			*numProbes,
			fmt.Sprintf("%s/probes_not_%s.dat", dataPrefix, *countryCode),
		)
	} else {
		nProbes := getNProbesNotCountry(*numProbes, *countryCode)
		for _, probe := range nProbes {
			nProbeIDs = append(nProbeIDs, fmt.Sprintf("%d", probe.ID))
		}
		infoLogger.Printf(
			"Writing Probe IDs to %s/probes_not_%s.dat\n",
			dataPrefix,
			*countryCode,
		)
		writeProbesToFile(
			fmt.Sprintf("%s/probes_not_%s.dat", dataPrefix, *countryCode),
			nProbeIDs,
		)
	}
	infoLogger.Printf("probe ids: %v\n", nProbeIDs)
}
