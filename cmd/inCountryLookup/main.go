package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	arg "github.com/alexflint/go-arg"
	atlas "github.com/keltia/ripe-atlas"
	probes "github.com/timartiny/RipeProbe/probes"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

type InCountryLookupFlags struct {
	CountryCode string `long:"country_code" description:"(Required) The Country Code to request probes from" required:"true" json:"country_code"`
	DomainFile  string `long:"domain_file" description:"(Required) Path to the file containing the domains to perform DNS lookups for, one domain per line" required:"true" json:"domain_file"`
	APIKey      string `long:"api_key" description:"(Required) Quote enclosed RIPE Atlas API key" required:"true" json:"api_key"`
	IDsFile     string `long:"ids_file" description:"(Required) Path to the file to write the RIPE Atlas measurement IDs to" required:"true" json:"ids_file"`
	GetProbes   bool   `long:"get_probes" description:"Whether to get new probes or not. If yes and probes_file is specified the probe ids will be written there" json:"get_probes"`
	ProbesFile  string `long:"probes_file" description:"If get_probes is specified this is the file to write out the probes used in this experiment if get_probes is not specified then this is the file to read probes from. If ommitted nothing is written" json:"probe_file"`
	NumProbes   int    `long:"num_probes" description:"Number of probes to do lookup with" default:"5" json:"num_probes"`
}

func setupArgs() InCountryLookupFlags {
	var ret InCountryLookupFlags
	arg.MustParse(&ret)

	return ret
}

// func simplifyProbeData(probeSlice []atlas.Probe) []probes.SimpleProbe {

// 	var miniDatas []probes.SimpleProbe
// 	for _, probe := range probeSlice {
// 		var miniData probes.SimpleProbe
// 		miniData.ID = probe.ID
// 		miniData.AddressV4 = probe.AddressV4
// 		miniData.PrefixV4 = probe.PrefixV4
// 		miniData.AddressV6 = probe.AddressV6
// 		miniData.PrefixV6 = probe.PrefixV6
// 		miniData.CountryCode = probe.CountryCode

// 		miniDatas = append(miniDatas, miniData)
// 	}

// 	return miniDatas
// }

func writeProbes(probeSlice []probes.SimpleProbe, writeFile string) {
	probeF, err := os.Create(writeFile)
	if err != nil {
		errorLogger.Fatalf("Couldn't create file: %v\n", err)
	}
	defer probeF.Close()

	for _, probe := range probeSlice {
		jsonMini, err := json.Marshal(probe)
		if err != nil {
			errorLogger.Printf("Error marshalling data: %v\n", err)
			continue
		}
		probeF.Write(jsonMini)
		probeF.WriteString("\n")
	}

	infoLogger.Printf("Wrote simplified data to %s\n", probeF.Name())
}

func getProbesFromFile(probeFile string) []probes.SimpleProbe {
	probeF, err := os.Open(probeFile)
	if err != nil {
		errorLogger.Fatalf("Error opening probe file, err: %v\n", err)
	}
	defer probeF.Close()

	var fullProbes []probes.SimpleProbe
	scanner := bufio.NewScanner(probeF)
	for scanner.Scan() {
		var probe probes.SimpleProbe
		jsonBytes := scanner.Text()
		err = json.Unmarshal([]byte(jsonBytes), &probe)
		if err != nil {
			errorLogger.Printf("Error unmarshalling probe data: %v\n", err)
			errorLogger.Fatalf("JSON data: %v\n", jsonBytes)
		}
		fullProbes = append(fullProbes, probe)
	}

	return fullProbes
}

func getProbesFromRIPE(countryCode, writeFile string) []probes.SimpleProbe {
	client, err := atlas.NewClient(atlas.Config{})
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}
	opts := make(map[string]string)
	opts["country_code"] = countryCode
	opts["status"] = "1"
	probeSlice, err := client.GetProbes(opts)
	if err != nil {
		errorLogger.Fatalf("Error getting probes, err: %v\n", err)
	}
	simplifiedProbes := probes.AtlasProbeSliceToSimpleProbeSlice(probeSlice)

	if len(writeFile) != 0 {
		writeProbes(simplifiedProbes, writeFile)
	}
	return simplifiedProbes
}

func makeDNSDefinitions(domains []string) []atlas.Definition {
	ret := make([]atlas.Definition, 0, len(domains))
	var selfResolve = true
	for _, domain := range domains {
		dns := atlas.Definition{
			Description:      "Local in-country DNS A lookup for " + domain,
			Type:             "dns",
			AF:               4,
			IsOneoff:         true,
			QueryClass:       "IN",
			QueryType:        "A",
			QueryArgument:    domain,
			ResolveOnProbe:   true,
			UseProbeResolver: selfResolve,
			SetRDBit:         true,
		}
		ret = append(ret, dns)
		dns = atlas.Definition{
			Description:      "Local in-country DNS AAAA lookup for " + domain,
			Type:             "dns",
			AF:               4, // Note this is asking what IP to do the lookup from
			IsOneoff:         true,
			QueryClass:       "IN",
			QueryType:        "AAAA",
			QueryArgument:    domain,
			ResolveOnProbe:   true,
			UseProbeResolver: selfResolve,
			SetRDBit:         true,
		}
		ret = append(ret, dns)
	}

	return ret
}

func atlasDNSLookup(domains []string, apiKey string, probeIds []string, startTime time.Time) ([]int, error) {
	if len(apiKey) <= 0 {
		errorLogger.Fatalf("need to provide an API key\n")
	}
	config := atlas.Config{
		APIKey: apiKey,
	}
	client, err := atlas.NewClient(config)
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}
	dnsDefinitions := makeDNSDefinitions(domains)

	probesString := strings.Join(probeIds, ",")
	dnsRequest := client.NewMeasurement()
	dnsRequest.Definitions = dnsDefinitions
	dnsRequest.Probes = []atlas.ProbeSet{
		{Requested: len(probeIds), Type: "probes", Value: probesString},
	}
	dnsRequest.StartTime = int(startTime.Unix())

	resp, err := client.DNS(dnsRequest)
	if err != nil {
		// errorLogger.Printf("%+v\n", dnsDefinitions)
		b, _ := json.Marshal(&dnsDefinitions)
		f, _ := os.Create("errorfile")
		f.Write(b)
		defer f.Close()
		errorLogger.Printf("Failed to create DNS measurements, err: %v\n", err)
		return []int{}, err
	}

	infoLogger.Printf(
		"Successfully created measurements, measurement IDs: %v\n",
		resp,
	)

	return resp.Measurements, nil
}

func saveIds(ids []int, idsFile string) {
	idFile, err := os.Create(idsFile)
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

func inCountryLookup(
	domainFile,
	apiKey string,
	probeSlice []probes.SimpleProbe,
	numProbes int,
	idsFile string,
) {
	domainF, err := os.Open(domainFile)
	if err != nil {
		errorLogger.Fatalf("Error opening domain file, err: %v\n", err)
	}
	defer domainF.Close()

	var domainList []string

	scanner := bufio.NewScanner(domainF)
	for scanner.Scan() {
		domainList = append(domainList, scanner.Text())
	}

	var probeIds []string

	for i := 0; i < numProbes; i++ {
		probeIds = append(probeIds, fmt.Sprint(probeSlice[i].ID))
	}

	infoLogger.Printf("Domains: %v, probes: %v\n", domainList, probeIds)

	startTime := time.Now()
	// RIPE Atlas works in multiples of 5 minutes, so go to the next multiple of
	// 5 minutes to give time for sending all requests
	startTime = startTime.Round(time.Minute * 5).Add(time.Minute * 5)
	measurementIds, err := atlasDNSLookup(
		domainList,
		apiKey,
		probeIds,
		startTime,
	)
	if err != nil {
		errorLogger.Fatalf("Error running experiment: %v\n", err)
	}
	infoLogger.Printf("Experiment scheduled it will run at %s\n", startTime.String())

	saveIds(measurementIds, idsFile)
}

func main() {
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

	args := setupArgs()

	var probeSlice []probes.SimpleProbe
	if args.GetProbes || len(args.ProbesFile) == 0 {
		infoLogger.Printf("Gathering live probes from %s\n", args.CountryCode)
		probeSlice = getProbesFromRIPE(args.CountryCode, args.ProbesFile)
	} else if len(args.ProbesFile) > 0 {
		probeSlice = getProbesFromFile(args.ProbesFile)
	} else {
		errorLogger.Fatal("Must provide either --get_probes or --probes_file " +
			"to run experiment",
		)
	}

	inCountryLookup(
		args.DomainFile, args.APIKey, probeSlice, args.NumProbes, args.IDsFile,
	)
}
