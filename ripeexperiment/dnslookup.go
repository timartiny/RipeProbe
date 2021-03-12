package ripeexperiment

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	atlas "github.com/keltia/ripe-atlas"
)

// LookupResult stores the results of a DNS lookup for a domain.
type LookupResult struct {
	Domain      string              `json:"domain"`
	Rank        int                 `json:"rank"`
	Source      string              `json:"source"`
	LocalV4     []string            `json:"local_v4_ips"`
	LocalV6     []string            `json:"local_v6_ips"`
	RipeResults []MeasurementResult `json:"ripe_results,omitempty"`
}

// MeasurementResult stores the addresses received from a RIPE measurement.
type MeasurementResult struct {
	ProbeID int      `json:"probe_id"`
	IDs     []int    `json:"ids"`
	V4      []string `json:"v4"`
	V6      []string `json:"v6"`
}

func makeDNSDefinitions(queries, targets []string) []atlas.Definition {
	ret := make([]atlas.Definition, 0, len(queries))
	var selfResolve bool

	if len(targets) > 0 {
		selfResolve = false
	} else {
		selfResolve = true
		targets = []string{""}
	}
	for _, domain := range queries {
		for _, target := range targets {
			var af int
			if strings.Index(target, ":") > -1 {
				af = 6
			} else {
				af = 4
			}
			dns := atlas.Definition{
				Description:      "DNS A lookup for " + domain,
				Type:             "dns",
				AF:               af,
				IsOneoff:         true,
				IsPublic:         false,
				QueryClass:       "IN",
				QueryType:        "A",
				Target:           target,
				QueryArgument:    domain,
				ResolveOnProbe:   true,
				UseProbeResolver: selfResolve,
			}
			ret = append(ret, dns)
			dns = atlas.Definition{
				Description:      "DNS AAAA lookup for " + domain,
				Type:             "dns",
				AF:               af,
				IsOneoff:         true,
				IsPublic:         false,
				QueryClass:       "IN",
				QueryType:        "AAAA",
				Target:           target,
				QueryArgument:    domain,
				ResolveOnProbe:   true,
				UseProbeResolver: selfResolve,
			}
			ret = append(ret, dns)
		}
	}

	return ret
}

// LookupAtlas uses apiKey to do DNS (A and AAAA) lookups for domains from
// probeIds
func LookupAtlas(queries []string, apiKey string, probeIds []string, targets []string) []int {
	config := atlas.Config{
		APIKey: apiKey,
	}
	client, err := atlas.NewClient(config)
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}
	dnsDefinitions := makeDNSDefinitions(queries, targets)

	probesString := strings.Join(probeIds, ",")
	dnsRequest := client.NewMeasurement()
	dnsRequest.Definitions = dnsDefinitions
	dnsRequest.Probes = []atlas.ProbeSet{
		{Requested: len(probeIds), Type: "probes", Value: probesString},
	}

	infoLogger.Printf("request: %v\n", dnsRequest)
	resp, err := client.DNS(dnsRequest)
	if err != nil {
		errorLogger.Fatalf("Faild to create DNS measurements, err: %v\n", err)
	}

	infoLogger.Printf(
		"Successfully created measurements, measurement IDs: %v\n",
		resp,
	)
	for _, id := range resp.Measurements {
		infoLogger.Printf(
			"to get response run:\n\tcurl -H \"Authorization: Key %s\" "+
				"https://atlas.ripe.net/api/v2/measurements/%d/results/ > "+
				"results.json\n",
			apiKey,
			id,
		)
	}

	return resp.Measurements
}

func lookup(record []string, data chan LookupResult, wg *sync.WaitGroup) {
	ipRecords, _ := net.LookupIP(record[1])
	rank, err := strconv.Atoi(record[0])
	if err != nil {
		// errorLogger.Printf(
		// 	"Error converting rank to int: %v, %v\n",
		// 	record[0],
		// 	err,
		// )
		rank = -1
	}
	result := LookupResult{
		Domain: record[1],
		Rank:   rank,
		Source: record[2],
	}
	for _, ip := range ipRecords {
		// fmt.Println(ip)
		if ip.To4() == nil {
			result.LocalV6 = append(result.LocalV6, ip.String())
		} else {
			result.LocalV4 = append(result.LocalV4, ip.String())
		}
	}

	data <- result
	wg.Done()
}

func writeDomain(data chan LookupResult, done chan bool, outPath string) {
	f, err := os.Create(outPath)
	if err != nil {
		done <- false
		errorLogger.Fatalf("can't open file, err: %v\n", err)
	}
	defer f.Close()

	f.WriteString("[")
	writeComma := false
	for domain := range data {
		if len(domain.LocalV4) == 0 || len(domain.LocalV6) == 0 {
			continue
		}
		if writeComma {
			f.WriteString(",")
		}
		jBytes, err := json.Marshal(&domain)
		if err != nil {
			errorLogger.Printf(
				"Error converting data to json: %v, %v\n",
				domain,
				err,
			)
		}
		f.Write(jBytes)
		writeComma = true
	}

	f.WriteString("]")
	done <- true
}

// LookupCSV reads domains from file and finds the ipaddr for each domain,
// records v6 ones.
func LookupCSV(csvPath, outPath string) {
	csvFile, err := os.Open(csvPath)
	if err != nil {
		errorLogger.Fatalf(
			"Could not open CSV file: %v, err: %v\n",
			csvPath,
			err,
		)
	}
	defer csvFile.Close()

	csvReader := csv.NewReader(csvFile)
	wg := sync.WaitGroup{}
	data := make(chan LookupResult)
	done := make(chan bool)

	//read title record
	_, err = csvReader.Read()
	if err != nil {
		errorLogger.Fatalf("error reading record, err: %v\n", err)
	}
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			errorLogger.Fatalf("error reading record, err: %v\n", err)
		}

		// fmt.Printf("record: %v\n", record[2])
		wg.Add(1)
		go lookup(record, data, &wg)
	}

	go writeDomain(data, done, outPath)

	go func() {
		wg.Wait()
		close(data)
	}()

	d := <-done

	if d == true {
		infoLogger.Printf("Wrote to %s successfully", outPath)
	} else {
		infoLogger.Println("Failed at writing to file")
	}
}
