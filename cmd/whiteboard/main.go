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
	experiment "github.com/timartiny/RipeProbe/RipeExperiment"
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
		file.WriteString(id + "\n")
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

func getProbeIDs(path, countryCode string, num int) []string {
	var ret []string
	if len(path) > 0 {
		ret = getNIDs(
			num,
			fmt.Sprintf("%s/probes_not_%s.dat", dataPrefix, countryCode),
		)
	} else {
		nProbes := getNProbesNotCountry(num, countryCode)
		for _, probe := range nProbes {
			ret = append(ret, fmt.Sprintf("%d", probe.ID))
		}
		infoLogger.Printf(
			"Writing Probe IDs to %s/probes_not_%s.dat\n",
			dataPrefix,
			countryCode,
		)
		writeProbesToFile(
			fmt.Sprintf("%s/probes_not_%s.dat", dataPrefix, countryCode),
			ret,
		)
	}

	return ret
}

func getListofStrings(path string) []string {
	var ret []string
	if len(path) <= 0 {
		errorLogger.Fatalf("Must provide path to resolver IPs, use -r")
	}

	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", path, err)
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		ret = append(ret, scanner.Text())
	}

	return ret
}

func getResolverIPs(path string) []string {
	return getListofStrings(path)
}

func getQueryDomains(path string) []string {
	return getListofStrings(path)
}

func saveIds(ids []int, timeStr string) {
	idFile, err := os.Create(dataPrefix + "/Whiteboard-Ids-" + timeStr)
	if err != nil {
		errorLogger.Fatalf(
			"error creating file to save measurements: %v\n",
			err,
		)
	}

	for _, id := range ids {
		idFile.WriteString(fmt.Sprintf("%d\n", id))
	}

}

func main() {
	dataPrefix = "../../data"
	numProbes := flag.Int("n", 0, "Number of probes to grab")
	countryCode := flag.String("c", "", "Country code to exclude from probes")
	probesPath := flag.String("p", "", "Path to file containing list of probe Ids")
	resolverIPsPath := flag.String("r", "", "Path to file containing the IPs to use as resolvers")
	queryDomainsPath := flag.String("q", "", "Path to file containing list of domains to do DNS queries from resolvers")
	apiKey := flag.String("apiKey", "", "RIPE Atlas API key")
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
	nProbeIDs := getProbeIDs(*probesPath, *countryCode, *numProbes)
	infoLogger.Printf("probe ids: %v\n", nProbeIDs)
	resolverIPs := getResolverIPs(*resolverIPsPath)
	infoLogger.Printf("Resolver IPs: %v\n", resolverIPs)
	queryDomains := getQueryDomains(*queryDomainsPath)
	infoLogger.Printf("Query Domains: %v\n", queryDomains)
	measurementIDs := experiment.LookupAtlas(queryDomains, *apiKey, nProbeIDs, resolverIPs)
	currentTime := time.Now()
	timeStr := fmt.Sprintf(
		"%d-%02d-%02d::%02d:%02d:%02d",
		currentTime.Year(),
		currentTime.Month(),
		currentTime.Day(),
		currentTime.Hour(),
		currentTime.Minute(),
		currentTime.Second(),
	)

	saveIds(measurementIDs, timeStr)
}
