package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	atlas "github.com/keltia/ripe-atlas"
	experiment "github.com/timartiny/RipeProbe/RipeExperiment"
)

const MAX_MEASUREMENTS = 100

var dataPrefix string
var infoLogger *log.Logger
var errorLogger *log.Logger
var SKIPCOUNTRIES []string

func getProbes() []atlas.Probe {
	client, err := atlas.NewClient(atlas.Config{})
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}
	opts := make(map[string]string)
	opts["status"] = "1"
	opts["prefix_v4"] = "0.0.0.0/0"
	opts["prefix_v6"] = "0:0:0:0:0:0:0:0/0"
	probes, err := client.GetProbes(opts)
	if err != nil {
		errorLogger.Fatalf("Error getting probes, err: %v\n", err)
	}
	return probes
}

func contains(l []string, s string) bool {
	for _, a := range l {
		if a == s {
			return true
		}
	}

	return false
}

func getNProbesNotCountry(size int) []atlas.Probe {
	var ret []atlas.Probe
	infoLogger.Printf("Grabbing all RIPE Atlas probes")
	allProbes := getProbes()
	numProbes := len(allProbes)
	s := rand.NewSource(time.Now().Unix())
	rg := rand.New(s)

	infoLogger.Printf(
		"Filtering down to %d probes not from specified countries, that have v4 and "+
			"v6 addresses\n", size,
	)
	for len(ret) < size {
		rInd := rg.Intn(numProbes)
		if !contains(SKIPCOUNTRIES, allProbes[rInd].CountryCode) {
			if len(allProbes[rInd].AddressV4) > 0 && len(allProbes[rInd].AddressV6) > 0 {
				ret = append(ret, allProbes[rInd])
			}
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

func getIDs(path string) []string {
	var ret []string
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", path, err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ret = append(ret, scanner.Text())
	}

	infoLogger.Printf("Returning %d probe ids\n", len(ret))

	return ret
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
		ret = getIDs(path)
	} else {
		nProbes := getNProbesNotCountry(num)
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
	var ret []string
	fullLines := getListofStrings(path)
	for _, line := range fullLines {
		split := strings.Split(line, " ")
		ret = append(ret, split[0])
	}

	return ret
}

func getQueryDomains(path string) []string {
	return getListofStrings(path)
}

func saveIds(ids []int, apiKey, timeStr, cc string) {
	idFile, err := os.Create(fmt.Sprintf("%s/Whiteboard-Ids-%s-%s", dataPrefix, cc, timeStr))
	if err != nil {
		errorLogger.Fatalf(
			"error creating file to save measurements: %v\n",
			err,
		)
	}
	infoLogger.Printf(
		"to get responses run:\n\t./fetchMeasurementResults.sh -a \"%s\" -f %s", apiKey, idFile.Name(),
	)

	for _, id := range ids {
		idFile.WriteString(fmt.Sprintf("%d\n", id))
	}

}

func batchDomains(fullList []string, size int) [][]string {
	var ret [][]string
	var loopI int

	for loopI = 0; loopI+size <= len(fullList); loopI += size {
		var tmp []string
		tmp = append(tmp, fullList[loopI:loopI+size]...)
		ret = append(ret, tmp)
	}

	if loopI != len(fullList) {
		var tmp []string
		tmp = append(tmp, fullList[loopI:]...)
		ret = append(ret, tmp)
	}

	return ret
}

func main() {
	SKIPCOUNTRIES = []string{"CN", "IR", "RU", "SA", "KR", "IN", "PK", "EG", "AR", "BR"}
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
	var measurementIDs []int
	startTime := time.Now().Add(time.Duration(time.Second * 30))
	startTime = startTime.Round(time.Minute * 5).Add(time.Minute * 5)
	// endTime := startTime.Add(time.Minute * 5)
	// measurementsPerDomain := len(resolverIPs) * 2
	domainsAtOnce := 1
	batches := batchDomains(queryDomains, domainsAtOnce)
	infoLogger.Printf(
		"To keep below %d measurements at once, we batch our domain queries. "+
			" We will query %d domains at once.\n",
		MAX_MEASUREMENTS,
		domainsAtOnce,
	)
	for _, batch := range batches {
		infoLogger.Printf("Scheduling experiment for %v, will start at %s\n", batch, startTime.String())
		ids, err := experiment.LookupAtlas(batch, *apiKey, nProbeIDs, resolverIPs, startTime)
		if err != nil {
			errorLogger.Printf("Got an error creating experiment\n")
			os.Exit(1)
		}
		measurementIDs = append(measurementIDs, ids...)
		startTime = startTime.Add(time.Minute * 20)
		// endTime = startTime.Add(time.Minute * 5)
	}
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

	saveIds(measurementIDs, *apiKey, timeStr, *countryCode)
}
