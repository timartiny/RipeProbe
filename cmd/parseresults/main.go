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

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

func getJSON(id int) []byte {
	if id == 0 {
		errorLogger.Fatalf(
			"Need a RIPE Atlas measurement id to parse, use --id",
		)
	}
	fileName := fmt.Sprintf("%s/%d_results.json", dataPrefix, id)
	file, err := os.Open(fileName)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", fileName, err)
	}
	defer file.Close()

	jsonBytes, err := ioutil.ReadAll(file)
	if err != nil {
		errorLogger.Fatalf("Error reading json file: %v\n", err)
	}

	return jsonBytes
}

func parseABuf(abuf string) {
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
		fmt.Printf("%s: %v\n", answer.Name, answer.IP)
	}
}

func parseResult(mResult results.MeasurementResult) {
	infoLogger.Printf("Parsing resultset for probe: %d\n", mResult.PrbID)
	for _, resultSet := range mResult.ResultSet {
		abuf := resultSet.Result.Abuf
		if len(abuf) <= 0 {
			infoLogger.Printf("No DNS answer, skipping\n")
			continue
		}
		parseABuf(abuf)
	}
}

func main() {
	dataPrefix = "../../data"
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

	jsonBytes := getJSON(*measID)

	var measResults []results.MeasurementResult
	json.Unmarshal(jsonBytes, &measResults)
	for _, res := range measResults {
		parseResult(res)
	}
}
