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

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	atlas "github.com/keltia/ripe-atlas"
	results "github.com/timartiny/RipeProbe/results"
)

var dataPrefix string
var infoLogger *log.Logger
var errorLogger *log.Logger

type IDtoResults map[string]results.ProbeResult

func getBytesByID(id string) []byte {
	file, err := os.Open(fmt.Sprintf("%s/%s_results.json", dataPrefix, id))
	if err != nil {
		errorLogger.Fatalf("Couldn't open file: %v\n", err)
	}

	ret, err := ioutil.ReadAll(file)
	if err != nil {
		errorLogger.Fatalf("Error reading bytes from file: %v\n", err)
	}

	return ret
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
		if fmt.Sprintf("%v", err) == "DNS packet too short" {
			return resMap
		} else {
			errorLogger.Fatalf("Failed to decode dns packet: %v\n", err)
		}
	}
	for _, answer := range dns.Answers {
		// infoLogger.Printf("%s: %v\n", answer.Name, answer.IP)
		resMap[string(answer.Name)] = append(
			resMap[string(answer.Name)], answer.IP.String(),
		)
	}

	return resMap
}

func getDetails(measResult results.MeasurementResult) results.WhiteboardResult {
	var det results.WhiteboardResult
	det.ProbeID = measResult.PrbID
	det.ProbeIP = measResult.From
	det.ResolverIP = measResult.DestAddr
	det.Queries = parseABuf(measResult.Result.Abuf)

	return det
}

func writeDetails(data IDtoResults, firstId, secondId string) {
	fName := fmt.Sprintf("%s/Whiteboard_results%s-%s.json", dataPrefix, firstId, secondId)
	file, err := os.Create(fName)
	if err != nil {
		errorLogger.Fatalf("Failed to create file %s: %v\n", fName, err)
	}
	defer file.Close()

	file.WriteString("[")
	for _, v := range data {
		detailsBytes, err := json.Marshal(&v)
		if err != nil {
			errorLogger.Fatalf("Error marshaling data: %v\n", err)
		}

		infoLogger.Printf("Writing bytes to %s\n", fName)
		file.Write(detailsBytes)
		file.WriteString("\n")
	}
	file.WriteString("]\n")

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

func addToQueryResult(qrs []results.QueryResult, newQR results.QueryResult) []results.QueryResult {
	for _, qr := range qrs {
		if qr.ResolverIP == newQR.ResolverIP {
			for newK, newV := range newQR.Queries {
				qr.Queries[newK] = append(qr.Queries[newK], newV...)
			}
			return qrs
		}
	}

	qrs = append(qrs, newQR)

	return qrs
}

func getFailedMeasurementData(id int) (string, string) {
	infoLogger.Printf("Measurement %d failed, looking up necessary info\n", id)
	config := atlas.Config{}
	client, err := atlas.NewClient(config)
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}

	resp, err := client.GetMeasurement(id)
	if err != nil {
		errorLogger.Printf("Error getting measurement %d, continuing", id)
	}
	split := strings.Split(resp.Description, " ")

	return split[1], split[len(split)-1]
}

func addToResult(currResult results.ProbeResult, newResults results.MeasurementResult) results.ProbeResult {
	if newResults.AF == 4 && currResult.V4Addr != newResults.From {
		currResult.V4Addr = newResults.From
	} else if newResults.AF == 6 && currResult.V6Addr != newResults.From {
		currResult.V6Addr = newResults.From
	}

	var queryRes results.QueryResult
	queryRes.ResolverIP = newResults.DestAddr
	if len(newResults.Error) > 0 {
		numAs, domain := getFailedMeasurementData(newResults.MsmID)
		queries := make(map[string][]string)
		queries[domain] = append(queries[domain], "")
		queryRes.Queries = queries
		if newResults.AF == 4 && len(numAs) == 1 {
			currResult.V4ToV4 = addToQueryResult(currResult.V4ToV4, queryRes)
		} else if newResults.AF == 4 && len(numAs) == 4 {
			currResult.V4ToV6 = addToQueryResult(currResult.V4ToV6, queryRes)
		} else if newResults.AF == 6 && len(numAs) == 1 {
			currResult.V6ToV4 = addToQueryResult(currResult.V6ToV4, queryRes)
		} else if newResults.AF == 6 && len(numAs) == 4 {
			currResult.V6ToV6 = addToQueryResult(currResult.V6ToV6, queryRes)
		} else {
			errorLogger.Printf("Error, should only have 4 cases here...")
		}

	} else {
		queries := parseABuf(newResults.Result.Abuf)
		queryRes.Queries = queries
		for _, v := range queries {
			if len(v) > 1 {
				errorLogger.Printf("had more than one result, not currently handled, %v\n", v)
				continue
			}
			if newResults.AF == 4 && strings.Index(v[0], ".") != -1 {
				currResult.V4ToV4 = addToQueryResult(currResult.V4ToV4, queryRes)
				break
			} else if newResults.AF == 4 && strings.Index(v[0], ":") != -1 {
				currResult.V4ToV6 = addToQueryResult(currResult.V4ToV6, queryRes)
				break
			} else if newResults.AF == 6 && strings.Index(v[0], ".") != -1 {
				currResult.V6ToV4 = addToQueryResult(currResult.V6ToV4, queryRes)
				break
			} else if newResults.AF == 6 && strings.Index(v[0], ":") != -1 {
				currResult.V6ToV6 = addToQueryResult(currResult.V6ToV6, queryRes)
				break
			} else {
				errorLogger.Printf("Error, should only have 4 cases here...")
				break
			}
		}
	}

	return currResult
}

func updateResults(currResults IDtoResults, id string) IDtoResults {
	measBytes := getBytesByID(id)
	var measResults []results.MeasurementResult
	err := json.Unmarshal(measBytes, &measResults)
	if err != nil {
		errorLogger.Fatalf("Error unmarshalling: %v\n", err)
	}
	if len(measResults) > 1 {
		errorLogger.Fatalf("more than one measurement results, look at this id: %s\n", id)
	}
	// measDetails := getDetails(measResults[0])
	// infoLogger.Printf("Measurement %s: %+v\n", id, measResults[0])
	strID := fmt.Sprintf("%d", measResults[0].PrbID)
	if i, ok := currResults[strID]; ok {
		currResults[strID] = addToResult(i, measResults[0])
	} else {
		var tmp results.ProbeResult
		tmp.ProbeID = measResults[0].PrbID
		currResults[strID] = addToResult(tmp, measResults[0])
	}

	return currResults
}

func main() {
	dataPrefix = "../../data"
	// measID := flag.Int("id", 0, "Measurement Id of results to parse, one at a time")
	measIDsFile := flag.String("m", "", "File containing all measurement IDs")
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

	ids := getMeasIDs(*measIDsFile)
	// fmt.Printf("ids: %v\n", ids)
	fullData := make(IDtoResults)
	for _, id := range ids {
		fullData = updateResults(fullData, id)
	}
	fmt.Printf("%+v\n", fullData)
	writeDetails(fullData, ids[0], ids[len(ids)-1])
}
