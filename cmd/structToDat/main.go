package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"

	experiment "github.com/timartiny/RipeProbe/RipeExperiment"
)

var dataPrefix string
var infoLogger *log.Logger
var errorLogger *log.Logger

func getStruct(path string) []experiment.LookupResult {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", path, err)
	}
	defer file.Close()

	structBytes, err := ioutil.ReadAll(file)
	if err != nil {
		errorLogger.Fatalf("Error reading bytes from file: %s, %v\n", path, err)
	}
	var res []experiment.LookupResult

	err = json.Unmarshal(structBytes, &res)
	if err != nil {
		errorLogger.Fatalf("Error unmarshalling bytes: %v\n", err)
	}

	return res
}

func strContains(arr []string, i string) bool {
	for _, a := range arr {
		if a == i {
			return true
		}
	}

	return false
}

func getIPs(structSlice []experiment.LookupResult) []string {
	var res []string
	for _, result := range structSlice {
		if len(result.RipeResults) > 0 {
			for _, ripeResult := range result.RipeResults {
				if len(ripeResult.V4) > 0 {
					if !strContains(res, ripeResult.V4[0]) {
						res = append(res, ripeResult.V4[0])
					}
				}
				if len(ripeResult.V6) > 0 {
					if !strContains(res, ripeResult.V6[0]) {
						res = append(res, ripeResult.V6[0])
					}
				}

			}
		}
	}

	return res
}

func writeIPs(ips []string, path string) {
	file, err := os.Create(path)
	if err != nil {
		errorLogger.Fatalf("Error creating outfile: %s, %v\n", path, err)
	}

	for _, ip := range ips {
		file.WriteString(ip + "\n")
	}
}

func main() {
	dataPrefix = "../../data"
	inFile := flag.String("i", "", "Path to file containing lookup results")
	outFile := flag.String("o", "", "Path to file to write ips to")
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

	structSlice := getStruct(*inFile)
	// infoLogger.Printf("StructSlice: %+v\n", structSlice)

	ips := getIPs(structSlice)
	writeIPs(ips, *outFile)
}
