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
var msmidToMetadata map[int][]string

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

func writeDetails(data IDtoResults, firstId, secondId string) {
	fName := fmt.Sprintf("%s/Whiteboard_results%s-%s.json", dataPrefix, firstId, secondId)
	file, err := os.Create(fName)
	if err != nil {
		errorLogger.Fatalf("Failed to create file %s: %v\n", fName, err)
	}
	defer file.Close()

	infoLogger.Printf("Writing bytes to %s\n", fName)
	file.WriteString("[")
	counter := 0
	for _, v := range data {
		detailsBytes, err := json.Marshal(&v)
		if err != nil {
			errorLogger.Fatalf("Error marshaling data: %v\n", err)
		}

		file.Write(detailsBytes)
		if counter != len(data)-1 {
			file.WriteString(",")
		}
		counter++
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
	var nAs, domain string
	if _, ok := msmidToMetadata[id]; !ok {
		infoLogger.Printf("Measurement %d had a failed query, looking up necessary info\n", id)
		config := atlas.Config{}
		client, err := atlas.NewClient(config)
		if err != nil {
			errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
		}

		resp, err := client.GetMeasurement(id)
		if err != nil {
			errorLogger.Printf("Error getting measurement %d, continuing", id)
			return "", ""
		}
		split := strings.Split(resp.Description, " ")
		nAs = split[1]
		domain = split[len(split)-1]
		msmidToMetadata[id] = []string{nAs, domain}
	} else {
		nAs = msmidToMetadata[id][0]
		domain = msmidToMetadata[id][1]
	}

	return nAs, domain
}

func addToResult(currResult results.ProbeResult, newResults results.MeasurementResult, resolverMap map[string]string) results.ProbeResult {
	if newResults.AF == 4 && currResult.V4Addr != newResults.From {
		currResult.V4Addr = newResults.From
	} else if newResults.AF == 6 && currResult.V6Addr != newResults.From {
		currResult.V6Addr = newResults.From
	}

	var queryRes results.QueryResult
	queryRes.ResolverIP = newResults.DestAddr
	queryRes.ResolverType = resolverMap[newResults.DestAddr]
	if len(newResults.Error) > 0 {
		numAs, domain := getFailedMeasurementData(newResults.MsmID)
		queries := make(map[string][]string)
		errorString := ""
		for k, v := range newResults.Error {
			if len(errorString) > 0 {
				errorString += ", "
			}
			errorString += fmt.Sprintf("%s: %v", k, v)
		}
		queries[domain] = append(queries[domain], errorString)
		queryRes.Queries = queries
		if strings.Contains(newResults.From, ".") && len(numAs) == 1 {
			currResult.V4ToV4 = addToQueryResult(currResult.V4ToV4, queryRes)
		} else if strings.Contains(newResults.From, ".") && len(numAs) == 4 {
			currResult.V4ToV6 = addToQueryResult(currResult.V4ToV6, queryRes)
		} else if strings.Contains(newResults.From, ":") && len(numAs) == 1 {
			currResult.V6ToV4 = addToQueryResult(currResult.V6ToV4, queryRes)
		} else if strings.Contains(newResults.From, ":") && len(numAs) == 4 {
			currResult.V6ToV6 = addToQueryResult(currResult.V6ToV6, queryRes)
		} else {
			errorLogger.Printf("Error, should only have 4 cases here...")
			errorLogger.Printf("newResults.AF = %d, len(numAs) = %d\n", newResults.AF, len(numAs))
			errorLogger.Printf("error: %v\n", newResults.Error)
		}

	} else {
		queries := parseABuf(newResults.Result.Abuf)
		queryRes.Queries = queries
		for _, v := range queries {
			// if len(v) > 1 {
			// 	errorLogger.Printf("had more than one result, not currently handled, %v\n", v)
			// 	continue
			// }
			if newResults.AF == 4 && strings.Contains(v[0], ".") {
				currResult.V4ToV4 = addToQueryResult(currResult.V4ToV4, queryRes)
				break
			} else if newResults.AF == 4 && strings.Contains(v[0], ":") {
				currResult.V4ToV6 = addToQueryResult(currResult.V4ToV6, queryRes)
				break
			} else if newResults.AF == 6 && strings.Contains(v[0], ".") {
				currResult.V6ToV4 = addToQueryResult(currResult.V6ToV4, queryRes)
				break
			} else if newResults.AF == 6 && strings.Contains(v[0], ":") {
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

func updateResults(currResults IDtoResults, id string, resolverMap map[string]string) IDtoResults {
	measBytes := getBytesByID(id)
	var measResults []results.MeasurementResult
	err := json.Unmarshal(measBytes, &measResults)
	if err != nil {
		errorLogger.Fatalf("Error unmarshalling: %v\n", err)
	}
	for _, indivResult := range measResults {
		strID := fmt.Sprintf("%d", indivResult.PrbID)
		if i, ok := currResults[strID]; ok {
			currResults[strID] = addToResult(i, indivResult, resolverMap)
		} else {
			var tmp results.ProbeResult
			tmp.ProbeID = indivResult.PrbID
			currResults[strID] = addToResult(tmp, indivResult, resolverMap)
		}
	}

	return currResults
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

func getResolvers(path string) map[string]string {
	ret := make(map[string]string)
	fullLines := getListofStrings(path)
	for _, line := range fullLines {
		split := strings.Split(line, " ")
		ret[split[0]] = split[1]
	}

	return ret
}

func main() {
	dataPrefix = "../../data"
	measIDsFile := flag.String("m", "", "File containing all measurement IDs")
	resolverFile := flag.String("r", "", "Path to file containing resolvers")
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
	resolverMap := getResolvers(*resolverFile)
	fullData := make(IDtoResults)
	// change dataPrefix to include folder for measurements
	dataPrefix += fmt.Sprintf("/%s-%s", ids[0], ids[len(ids)-1])
	msmidToMetadata = make(map[int][]string)
	for _, id := range ids {
		fullData = updateResults(fullData, id, resolverMap)
	}
	// fmt.Printf("%+v\n", fullData)
	writeDetails(fullData, ids[0], ids[len(ids)-1])
}
