package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	atlas "github.com/keltia/ripe-atlas"
	experiment "github.com/timartiny/RipeProbe/RipeExperiment"
)

var dataFilePrefix string

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

func fillIn(probe atlas.Probe) experiment.ProbeIPs {
	var miniData experiment.ProbeIPs
	miniData.ID = probe.ID
	miniData.AddressV4 = probe.AddressV4
	miniData.PrefixV4 = probe.PrefixV4
	miniData.AddressV6 = probe.AddressV6
	miniData.PrefixV6 = probe.PrefixV6

	return miniData
}

func writeProbes(probes []atlas.Probe, countryCode string) {
	probesBytes, err := json.MarshalIndent(probes, "", "\t")
	if err != nil {
		errorLogger.Fatalf("Error unmarshalling probes, err: %v\n", err)
	}
	f, err := os.Create(
		fmt.Sprintf("%s/%s_full_probe_data.json", dataFilePrefix, countryCode),
	)
	if err != nil {
		errorLogger.Fatalf("Couldn't open file: %v\n", err)
	}
	defer f.Close()

	_, err = f.Write(probesBytes)
	if err != nil {
		errorLogger.Fatalf("Couldn't write to file: %v\n", err)
	}
	f.WriteString("\n")
	infoLogger.Printf("Wrote full response to %s\n", f.Name())

	probeF, err := os.Create(
		fmt.Sprintf("%s/%s_probes.json", dataFilePrefix, countryCode),
	)
	if err != nil {
		errorLogger.Fatalf("Couldn't open file: %v\n", err)
	}
	defer probeF.Close()

	var miniProbes []experiment.ProbeIPs
	for _, probe := range probes {
		miniProbes = append(miniProbes, fillIn(probe))
	}
	jsonMini, _ := json.MarshalIndent(miniProbes, "", "\t")
	probeF.Write(jsonMini)
	probeF.WriteString("\n")

	infoLogger.Printf("Wrote simplified data to %s\n", probeF.Name())
}

func getProbes(countryCode string) {
	if countryCode == "" {
		errorLogger.Fatalf(
			"To gather probes must enter Countrycode, using flag -c",
		)
	}

	probes := experiment.GetProbes(countryCode)
	writeProbes(probes, countryCode)
}

func atlasExperiment(domainFile, apiKey, probeFile string) {
	if len(apiKey) <= 0 {
		errorLogger.Fatalf(
			"To do atlas experiments you need to provide an API key " +
				"with --apiKey",
		)
	}
	f, err := os.Open(domainFile)
	if err != nil {
		errorLogger.Fatalf("Error opening CSV file, err: %v\n", err)
	}
	defer f.Close()

	var domainList []string

	jBytes, err := ioutil.ReadAll(f)
	if err != nil {
		errorLogger.Fatalf("Can't read bytes from %s, %v\n", domainFile, err)
	}
	var records []experiment.LookupResult
	err = json.Unmarshal(jBytes, &records)
	if err != nil {
		errorLogger.Fatalf("Can't unmarshal json bytes, %v", err)
	}
	for _, record := range records {
		domainList = append(domainList, record.Domain)
	}

	probeF, err := os.Open(probeFile)
	if err != nil {
		errorLogger.Fatalf("Error opening CSV file, err: %v\n", err)
	}
	defer probeF.Close()

	var fullProbes []experiment.ProbeIPs
	jsonBytes, err := ioutil.ReadAll(probeF)
	if err != nil {
		errorLogger.Fatalf("Error reading JSON file: %v\n", err)
	}

	err = json.Unmarshal(jsonBytes, &fullProbes)
	if err != nil {
		errorLogger.Fatalf("Error unmarshalling JSON file: %v\n", err)
	}

	var probeIds []string

	for i := 0; i < 5; i++ {
		probeIds = append(probeIds, fmt.Sprint(fullProbes[i].ID))
	}

	infoLogger.Printf("Domains: %v, probes: %v\n", domainList, probeIds)

	startTime := time.Now().Add(time.Duration(time.Second * 30))
	startTime = startTime.Round(time.Minute * 5).Add(time.Minute * 5)
	measurementIds, err := experiment.LookupAtlas(domainList, apiKey, probeIds, []string{}, startTime)
	if err != nil {
		errorLogger.Fatalf("Error running experiment: %v\n", err)
	}
	infoLogger.Printf("Experiment scheduled it will run at %s\n", startTime.String())
	timeStr := fmt.Sprintf(
		"%d-%02d-%02d::%02d:%02d",
		startTime.Year(),
		startTime.Month(),
		startTime.Day(),
		startTime.Hour(),
		startTime.Minute(),
	)

	saveIds(measurementIds, timeStr)
}

func saveIds(ids []int, timeStr string) {
	idFile, err := os.Create(dataFilePrefix + "/inCountryLookup-Ids-" + timeStr)
	if err != nil {
		errorLogger.Fatalf(
			"error creating file to save measurements: %v\n",
			err,
		)
	}
	defer idFile.Close()

	infoLogger.Printf("Saving measurement IDs to %s\n", idFile.Name())

	infoLogger.Printf("To retrieve results run fetchMeasurementResults in main directory\n")

	for _, id := range ids {
		idFile.WriteString(fmt.Sprintf("%d\n", id))
	}

}

func main() {
	dataFilePrefix = "data"
	countryCode := flag.String(
		"c",
		"",
		"The Country Code to request probes from",
	)
	probeFile := flag.String(
		"probeFile",
		"",
		"JSON file of probes that can be provided instead of doing a live lookup",
	)
	lookupFile := flag.String(
		"lookupFile",
		"",
		"JSON file containing domains to lookup with RIPE Atlas, otherwise "+
			"uses Country Code for default file",
	)
	noAtlasFlag := flag.Bool(
		"noatlas",
		false,
		"Will just do a dry run, won't actually call out to RIPE Atlas",
	)
	apiKey := flag.String("apiKey", "", "API key as string")

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
	if len(*lookupFile) == 0 {
		*lookupFile = fmt.Sprintf(
			"%s/%s_lookup.json", dataFilePrefix, *countryCode,
		)
	}
	if len(*probeFile) == 0 {
		*probeFile = fmt.Sprintf(
			"%s/%s_probes.json", dataFilePrefix, *countryCode,
		)
		infoLogger.Printf("Gathering live probes from %s\n", *countryCode)
		getProbes(*countryCode)
	}

	if !*noAtlasFlag {
		atlasExperiment(*lookupFile, *apiKey, *probeFile)
	}
}
