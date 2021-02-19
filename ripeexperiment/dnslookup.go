package ripeexperiment

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	atlas "github.com/keltia/ripe-atlas"
)

// LookupAtlas uses apiKey to do DNS (A and AAAA) lookups for domains from
// probeIds
func LookupAtlas(domains []string, apiKey string, probeIds []string) []int {
	config := atlas.Config{
		APIKey: apiKey,
	}
	client, err := atlas.NewClient(config)
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}
	dnsDefinitions := make([]atlas.Definition, 0, len(domains))

	for _, domain := range domains {
		dns := atlas.Definition{
			Description:      "DNS A lookup for " + domain,
			Type:             "dns",
			AF:               4,
			IsOneoff:         true,
			IsPublic:         false,
			QueryClass:       "IN",
			QueryType:        "A",
			QueryArgument:    domain,
			ResolveOnProbe:   true,
			UseProbeResolver: true,
		}
		dnsDefinitions = append(dnsDefinitions, dns)
		dns = atlas.Definition{
			Description:      "DNS A lookup for " + domain,
			Type:             "dns",
			AF:               6,
			IsOneoff:         true,
			IsPublic:         false,
			QueryClass:       "IN",
			QueryType:        "AAAA",
			QueryArgument:    domain,
			ResolveOnProbe:   true,
			UseProbeResolver: true,
		}
		dnsDefinitions = append(dnsDefinitions, dns)
	}
	probesString := strings.Join(probeIds, ",")
	dnsRequest := client.NewMeasurement()
	dnsRequest.Definitions = dnsDefinitions
	dnsRequest.Probes = []atlas.ProbeSet{
		{Requested: len(probeIds), Type: "probes", Value: probesString},
	}

	resp, err := client.DNS(dnsRequest)
	if err != nil {
		errorLogger.Fatalf("Faild to create DNS measurements, err: %v\n", err)
	}

	infoLogger.Printf(
		"Successfully created measurements, measurement IDs: %v\n",
		resp,
	)
	for id := range resp.Measurements {
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

func lookup(record []string, data chan string, wg *sync.WaitGroup) {
	ipRecords, _ := net.LookupIP(record[1])
	for _, ip := range ipRecords {
		// fmt.Println(ip)
		if ip.To4() == nil || record[2] == "OONI" || record[2] == "Alexa" {
			data <- fmt.Sprintf(
				"%s\t%s\t%s\t%s", record[0], record[1], record[2], ip.String(),
			)
			break
		}
	}

	wg.Done()
}

func writeDomain(data chan string, done chan bool, outPath string) {
	f, err := os.Create(outPath)
	if err != nil {
		done <- false
		errorLogger.Fatalf("can't open file, err: %v\n", err)
	}
	defer f.Close()

	csvWriter := csv.NewWriter(f)

	err = csvWriter.Write(
		[]string{"Rank", "Domain", "Source", "Local Address"},
	)
	if err != nil {
		done <- false
		errorLogger.Fatalf("Can't write to file, err: %v\n", err)
	}
	csvWriter.Flush()
	for domain := range data {
		split := strings.Split(domain, "\t")
		// _, err = f.WriteString(domain + "\n")
		err = csvWriter.Write(split)
		if err != nil {
			done <- false
			errorLogger.Fatalf("Can't write to file, err: %v\n", err)
		}
		csvWriter.Flush()
	}

	csvWriter.Flush()
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
	data := make(chan string)
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
		infoLogger.Println("Wrote to file successfully")
	} else {
		infoLogger.Println("Failed at writing to file")
	}
}
