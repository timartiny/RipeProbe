package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
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

func addOONI(csvPath, jsonPath, alexaPath string) {
	csvFile, err := os.Open(csvPath)
	if err != nil {
		errorLogger.Fatalf("error opening csv file, %s: %v\n", csvPath, err)
	}

	reader := csv.NewReader(csvFile)
	allRecords, err := reader.ReadAll()
	if err != nil {
		errorLogger.Fatalf("error reading csv file, %v\n", err)
	}

	csvFile.Close()

	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		errorLogger.Fatalf("error opening JSON file, %s: %v\n", jsonPath, err)
	}
	defer jsonFile.Close()

	jsonBytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		errorLogger.Fatalf("error reading JSON file, %v\n", err)
	}
	var actualJSON map[string]interface{}
	json.Unmarshal(jsonBytes, &actualJSON)

	var count int
	var ooniDomains []string

	ooniReplace := map[string]string{
		"facebook_messenger": "facebook.com",
		"whatsapp":           "web.whatsapp.com",
		"telegram":           "web.telegram.org",
	}
	for key := range actualJSON {
		replace, ok := ooniReplace[key]
		if ok {
			key = replace
		}
		protoIndex := strings.Index(key, "://")
		hostAndPath := key
		if protoIndex != -1 {
			hostAndPath = key[protoIndex+3:]
		}
		wwwIndex := strings.Index(hostAndPath, "www.")
		if wwwIndex != -1 {
			hostAndPath = hostAndPath[wwwIndex+4:]
		}
		hostAndPathLen := len(hostAndPath)
		if hostAndPathLen > 0 && hostAndPath[hostAndPathLen-1] == '/' {
			hostAndPath = hostAndPath[:hostAndPathLen-1]
		}
		ooniDomains = append(ooniDomains, hostAndPath)
		count++
	}
	infoLogger.Printf("OONI domains: %v\n", ooniDomains)

	alexaFile, err := os.Open(alexaPath)
	if err != nil {
		errorLogger.Fatalf("error opening Alexa file, %s: %v\n", alexaPath, err)
	}
	defer alexaFile.Close()

	var alexaDomains []string
	scanner := bufio.NewScanner(alexaFile)

	for scanner.Scan() {
		alexaDomains = append(alexaDomains, scanner.Text())
	}

	infoLogger.Printf("Alexa domains: %v\n", alexaDomains)

	// now write back to csvPath with some header and source info and OONI domains
	csvFile, err = os.Create(csvPath)
	if err != nil {
		errorLogger.Fatalf("Error opening csv file for writing, %v\n", err)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)

	err = writer.Write([]string{"Rank", "Domain", "Source"})
	if err != nil {
		errorLogger.Fatalf("Error writing to file: %v\n", err)
	}

	for _, record := range allRecords {
		record = append(record, "Tranco/CitizenLab")
		err = writer.Write(record)
		if err != nil {
			errorLogger.Fatalf("Error writing to file: %v\n", err)
		}
		writer.Flush()
	}

	for _, domain := range ooniDomains {
		record := []string{"-", domain, "OONI"}
		err = writer.Write(record)
		if err != nil {
			errorLogger.Fatalf("Error writing to file: %v\n", err)
		}
		writer.Flush()
	}

	for _, domain := range alexaDomains {
		record := []string{"-", domain, "Alexa"}
		err = writer.Write(record)
		if err != nil {
			errorLogger.Fatalf("Error writing to file: %v\n", err)
		}
		writer.Flush()
	}
	writer.Flush()
}

func intersectCSVs(if1, if2, of string, size int) {
	infoLogger.Printf(
		"Will intersect entries of %s and %s and store intersection "+
			"(of size %d) in %s\n",
		if1,
		if2,
		size,
		of,
	)
	if _, err := os.Stat(if2); os.IsNotExist(err) {
		errorLogger.Printf(
			"File %s does not exist, using %s/global.csv as default\n",
			if2,
			dataFilePrefix,
		)
		if2 = fmt.Sprintf("%s/global.csv", dataFilePrefix)
	}
	err := experiment.IntersectCSV(if1, if2, of, size)
	if err != nil {
		errorLogger.Fatalf(
			"Error intersecting files, will stop execution: %v\n",
			err,
		)
	}

}

func lookupCSV(domainPath, outPath string) {
	infoLogger.Printf(
		"Looking for v6 addresses of domains from %s. Storing results in %s\n",
		domainPath,
		outPath,
	)
	experiment.LookupCSV(domainPath, outPath)
}

func atlasExperiment(domainFile, apiKey, probeFile, timeStr string) {
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

	// for _, probe := range fullProbes{
	for i := 0; i < 5; i++ {
		probeIds = append(probeIds, fmt.Sprint(fullProbes[i].ID))
		// probeIds = append(probeIds, probe.ID)
	}

	infoLogger.Printf("Domains: %v, probes: %v\n", domainList, probeIds)

	startTime := time.Now().Add(time.Duration(time.Second * 30))
	startTime = startTime.Round(time.Minute * 5).Add(time.Minute * 5)
	measurementIds, err := experiment.LookupAtlas(domainList, apiKey, probeIds, []string{}, startTime)
	if err != nil {
		errorLogger.Fatalf("Error running experiment: %v\n", err)
	}
	infoLogger.Printf("Experiment scheduled it will run at %s\n", startTime.String())

	saveIds(measurementIds, timeStr)
}

func saveIds(ids []int, timeStr string) {
	idFile, err := os.Create(dataFilePrefix + "/Ids-" + timeStr)
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

func datToCSV(datFile, csvFile string) {
	dF, err := os.Open(datFile)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", datFile, err)
	}
	defer dF.Close()

	cF, err := os.Create(csvFile)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", csvFile, err)
	}
	defer cF.Close()

	domainScanner := bufio.NewScanner(dF)
	csvWriter := csv.NewWriter(cF)
	csvWriter.Write([]string{"Rank", "Domain", "Source"})

	for domainScanner.Scan() {
		csvWriter.Write([]string{"-", domainScanner.Text(), "Manual"})
		csvWriter.Flush()
	}
	csvWriter.Flush()
}

func main() {
	dataFilePrefix = "data"
	countryCode := flag.String(
		"c",
		"",
		"The Country Code to request probes from",
	)
	noProbeFlag := flag.Bool(
		"noprobe",
		false,
		"Will stop script from looking for probes from given country",
	)
	noIntersectFlag := flag.Bool(
		"nointersect",
		false,
		"Will stop script from intersecting CitizenLab and Tranco CSV files. "+
			"Future steps will assume intersection.csv exists",
	)
	intersectSize := flag.Int(
		"intersectsize",
		50,
		"Desired size of the intersection",
	)
	noAddExtraDomainsFlag := flag.Bool(
		"noExtraDomains",
		false,
		"Will stop the script from adding Alexa and OONI domains.",
	)
	noLookupFlag := flag.Bool(
		"nolookup",
		false,
		"Will stop script from looking up whether intersection file has v6 "+
			"addresses. Future steps will assume lookup.csv exists",
	)
	noAtlasFlag := flag.Bool(
		"noatlas",
		false,
		"Will stop script from looking up v6 addresses from RIPE Atlas, "+
			"using probe list",
	)
	setDomainsFile := flag.String(
		"setDomains",
		"",
		"Will skip most of the steps in this script and use a file of "+
			"domains, passed through this command",
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
	intersectionFile := fmt.Sprintf("%s/%s_intersection.csv", dataFilePrefix, *countryCode)
	trancoList := fmt.Sprintf("%s/top-1m.csv", dataFilePrefix)
	citizenLabList := strings.ToLower(
		fmt.Sprintf("%s/%s.csv", dataFilePrefix, *countryCode),
	)
	ooniFile := fmt.Sprintf("%s/%s_OONI.json", dataFilePrefix, *countryCode)
	alexaFile := fmt.Sprintf("%s/%s_Alexa.dat", dataFilePrefix, *countryCode)
	lookupFile := fmt.Sprintf(
		"%s/%s_lookup.json",
		dataFilePrefix,
		*countryCode,
	)
	probeFile := fmt.Sprintf("%s/%s_probes.json", dataFilePrefix, *countryCode)
	if !*noProbeFlag {
		infoLogger.Printf("Gathering live probes from %s\n", *countryCode)
		getProbes(*countryCode)
	}

	if len(*setDomainsFile) != 0 {
		datToCSV(
			*setDomainsFile,
			intersectionFile,
		)
	} else {
		if !*noIntersectFlag {
			intersectCSVs(
				trancoList,
				citizenLabList,
				intersectionFile,
				*intersectSize,
			)
		}

		if !*noAddExtraDomainsFlag {
			infoLogger.Println("Now add in OONI and Alexa domains")
			addOONI(
				intersectionFile,
				ooniFile,
				alexaFile,
			)
		}
	}

	if !*noLookupFlag {
		lookupCSV(
			intersectionFile,
			lookupFile,
		)
	}

	if !*noAtlasFlag {
		atlasExperiment(
			lookupFile,
			*apiKey,
			probeFile,
			timeStr,
		)
	}
}
