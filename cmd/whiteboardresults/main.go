package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	results "github.com/timartiny/RipeProbe/results"
)

var dataPrefix string
var infoLogger *log.Logger
var errorLogger *log.Logger

func getBytesByID(id int) []byte {
	file, err := os.Open(fmt.Sprintf("%s/%d_results.json", dataPrefix, id))
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
		errorLogger.Fatalf("Failed to decode dns packet: %v\n", err)
	}
	for _, answer := range dns.Answers {
		// infoLogger.Printf("%s: %v\n", answer.Name, answer.IP)
		resMap[string(answer.Name)] = append(
			resMap[string(answer.Name)], answer.IP.String(),
		)
	}

	return resMap
}

func getDetails(fullresults []results.MeasurementResult) []results.WhiteboardResult {
	var ret []results.WhiteboardResult
	for _, measResult := range fullresults {
		var det results.WhiteboardResult
		det.ProbeID = measResult.PrbID
		det.ProbeIP = measResult.From
		det.ResolverIP = measResult.DestAddr
		det.Queries = parseABuf(measResult.Result.Abuf)
		ret = append(ret, det)
	}

	return ret
}

func writeDetails(fullResults []results.MeasurementResult, id int) {
	fName := fmt.Sprintf("%s/%d_details.json", dataPrefix, id)
	file, err := os.Create(fName)
	if err != nil {
		errorLogger.Fatalf("Failed to create file %s: %v\n", fName, err)
	}
	defer file.Close()

	details := getDetails(fullResults)

	detailsBytes, err := json.Marshal(&details)
	if err != nil {
		errorLogger.Fatalf("Unable to marshal details: %+v\n%v\n", details, err)
	}

	infoLogger.Printf("Writing bytes to %s\n", fName)
	file.Write(detailsBytes)
	file.WriteString("\n")
}

func main() {
	dataPrefix = "data"
	measID := flag.Int("id", 0, "Measurement Id of results to parse, one at a time")
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

	measBytes := getBytesByID(*measID)
	var measResults []results.MeasurementResult
	json.Unmarshal(measBytes, &measResults)
	writeDetails(measResults, *measID)
}
