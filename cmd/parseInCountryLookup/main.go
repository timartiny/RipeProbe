package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
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

type ParseInCountryLookupFlags struct {
	CountryCode string `arg:"--country_code,required" help:"(Required) The Country Code to request probes from" json:"country_code"`
	DomainFile  string `arg:"--domain_file,required" help:"(Required) Path to the file containing the domains to perform DNS lookups for, one domain per line" json:"domain_file"`
	APIKey      string `arg:"--api_key,required" help:"(Required) Quote enclosed RIPE Atlas API key" json:"api_key"`
	IDsFile     string `arg:"--ids_file,required" help:"(Required) Path to the file to write the RIPE Atlas measurement IDs to" json:"ids_file"`
	GetProbes   bool   `arg:"--get_probes" help:"Whether to get new probes or not. If yes and probes_file is specified the probe ids will be written there" json:"get_probes"`
	ProbesFile  string `arg:"--probes_file" help:"If get_probes is specified this is the file to write out the probes used in this experiment if get_probes is not specified then this is the file to read probes from. If ommitted nothing is written" json:"probe_file"`
	NumProbes   int    `arg:"--num_probes" help:"Number of probes to do lookup with" default:"5" json:"num_probes"`
}

func setupArgs() ParseInCountryLookupFlags {
	var ret ParseInCountryLookupFlags
	arg.MustParse(&ret)

	return ret
}

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

func writeJSON(path string, lookup []experiment.LookupResult) {
	file, err := os.Create(path)
	if err != nil {
		errorLogger.Fatalf("Error truncating file: %s, %v\n", path, err)
	}
	defer file.Close()

	jsonBytes, err := json.Marshal(&lookup)
	if err != nil {
		errorLogger.Fatalf("Error turning lookup into bytes: %v\n", err)
	}

	file.Write(jsonBytes)
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
		res[result.Domain] = i + 1
	}

	return res
}

func findResult(rr []experiment.MeasurementResult, res results.MeasurementResult) (experiment.MeasurementResult, int) {
	for i, r := range rr {
		if r.ProbeID == res.PrbID {
			if !intContains(r.IDs, res.MsmID) {
				r.IDs = append(r.IDs, res.MsmID)
			}
			return r, i
		}
	}

	return experiment.MeasurementResult{}, -1
}

func intContains(arr []int, i int) bool {
	for _, a := range arr {
		if a == i {
			return true
		}
	}

	return false
}

func strContains(arr []string, i string) bool {
	for _, a := range arr {
		if a == i {
			return true
		}
	}

	return false
}

func getMeasIDs(path string) []string {
	var ret []string
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Error opening measurement Id file, %s: %v\n", path, err)
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		ret = append(ret, scanner.Text())
	}

	return ret
}

func main() {
	dataPrefix = "data"
	measIDPath := flag.String("ids", "", "Path to measurement IDs file")
	inJSONPath := flag.String("in", "", "Path to JSON file that has lookup domains for associated measurement ID")
	outJSONPath := flag.String("out", "", "Path to where to put full JSON results")
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

	lookupBytes := getJSON(*inJSONPath)
	var lookup []experiment.LookupResult
	json.Unmarshal(lookupBytes, &lookup)
	// fmt.Printf("%+v\n", lookup)
	lookupDomains := getDomains(lookup)
	ids := getMeasIDs(*measIDPath)

	dataPrefix += fmt.Sprintf("/%s-%s", ids[0], ids[len(ids)-1])
	for _, measID := range ids {
		measBytes := getJSON(fmt.Sprintf("%s/%s_results.json", dataPrefix, measID))
		var measResults []results.MeasurementResult
		json.Unmarshal(measBytes, &measResults)
		for _, res := range measResults {
			answers := parseResult(res)
			for _, answer := range answers {
				for domain, ipSlice := range answer {
					var measResult experiment.MeasurementResult
					measIndex := -1
					infoLogger.Printf("%s:\t%s\n", domain, ipSlice)
					if lookupDomains[domain] > 0 {
						if len(lookup[lookupDomains[domain]-1].RipeResults) > 0 {
							measResult, measIndex = findResult(lookup[lookupDomains[domain]-1].RipeResults, res)
						}
						if measIndex == -1 {
							if !intContains(measResult.IDs, res.MsmID) {
								measResult.IDs = append(measResult.IDs, res.MsmID)
							}
							measResult.ProbeID = res.PrbID
						}
						for _, ip := range ipSlice {
							if strings.Contains(ip, ".") {
								if !strContains(measResult.V4, ip) {
									measResult.V4 = append(measResult.V4, ip)
								}
							} else if strings.Contains(ip, ":") {
								if !strContains(measResult.V6, ip) {
									measResult.V6 = append(measResult.V6, ip)
								}
							}
						}
					}
					if len(measResult.V4) > 0 || len(measResult.V6) > 0 {
						if measIndex == -1 {
							lookup[lookupDomains[domain]-1].RipeResults = append(
								lookup[lookupDomains[domain]-1].RipeResults, measResult,
							)
						} else {
							lookup[lookupDomains[domain]-1].RipeResults[measIndex] =
								measResult
						}
					}
				}
			}
		}
	}
	// fmt.Printf("%+v\n", lookup)
	writeJSON(*outJSONPath, lookup)
}
