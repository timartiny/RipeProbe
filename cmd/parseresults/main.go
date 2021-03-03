package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	experiment "github.com/timartiny/RipeProbe/RipeExperiment"
	results "github.com/timartiny/RipeProbe/results"
)

var dataPrefix string

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

func getJSON(path string) []byte {
	if len(path) == 0 {
		errorLogger.Fatalf("Need a file name to parse")
	}
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", path, err)
	}
	defer file.Close()

	jsonBytes, err := ioutil.ReadAll(file)
	if err != nil {
		errorLogger.Fatalf("Error reading json file: %v\n", err)
	}

	return jsonBytes
}

func parseABuf(abuf string) map[string][]string {
	resMap := make(map[string][]string)
	resBytes, err := base64.StdEncoding.DecodeString(abuf)
	if err != nil {
		errorLogger.Fatalf("Error decoding base64 str: %s, %v\n", abuf, err)
	}
	dns := &layers.DNS{}

	err = dns.DecodeFromBytes(resBytes, gopacket.NilDecodeFeedback)
	if err != nil {
		errorLogger.Fatalf("Failed to decode dns packet: %v\n", err)
	}
	for _, answer := range dns.Answers {
		// fmt.Printf("%s: %v\n", answer.Name, answer.IP)
		resMap[string(answer.Name)] = append(
			resMap[string(answer.Name)], answer.IP.String(),
		)
	}

	return resMap
}

func parseResult(mResult results.MeasurementResult) []map[string][]string {
	res := make([]map[string][]string, 0)
	infoLogger.Printf("Parsing resultset for probe: %d\n", mResult.PrbID)
	for _, resultSet := range mResult.ResultSet {
		abuf := resultSet.Result.Abuf
		if len(abuf) <= 0 {
			infoLogger.Printf("No DNS answer, skipping\n")
			continue
		}
		res = append(res, parseABuf(abuf))
	}

	return res
}

func getDomains(results []experiment.LookupResult) map[string]int {
	res := make(map[string]int)
	for i, result := range results {
		res[result.Domain] = i
	}

	return res
}

func main() {
	dataPrefix = "../../data"
	measID := flag.Int("id", 0, "Measurement Id of results to parse, one at a time")
	jsonPath := flag.String("f", "", "Path to JSON file that has lookup domains for associated measurement ID")
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

	lookupBytes := getJSON(*jsonPath)
	var lookup []experiment.LookupResult
	json.Unmarshal(lookupBytes, &lookup)
	fmt.Printf("%+v\n", lookup)
	lookupDomains := getDomains(lookup)

	measBytes := getJSON(fmt.Sprintf("%s/%d_results.json", dataPrefix, *measID))
	var measResults []results.MeasurementResult
	json.Unmarshal(measBytes, &measResults)
	for _, res := range measResults {
		answers := parseResult(res)
		for _, answer := range answers {
			for domain, ipSlice := range answer {
				var measResult experiment.MeasurementResult
				measResult.ID = res.MsmID
				measResult.ProbeID = res.PrbID
				fmt.Printf("%s:\t%s\n", domain, ipSlice)
				if lookupDomains[domain] > 0 {
					for _, ip := range ipSlice {
						if strings.Index(ip, ".") != -1 {
							measResult.V4 = append(measResult.V4, ip)
						} else if strings.Index(ip, ":") != -1 {
							measResult.V6 = append(measResult.V6, ip)
						}
					}
				}
				if len(measResult.V4) > 0 || len(measResult.V6) > 0 {
					lookup[lookupDomains[domain]].RipeResults = append(
						lookup[lookupDomains[domain]].RipeResults, measResult,
					)
				}
			}
		}
	}
	fmt.Printf("%+v\n", lookup)
}
