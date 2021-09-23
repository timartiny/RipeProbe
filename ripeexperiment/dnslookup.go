package ripeexperiment

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	atlas "github.com/keltia/ripe-atlas"
)

// LookupResult stores the results of a DNS lookup for a domain.
type LookupResult struct {
	Domain      string              `json:"domain"`
	Rank        int                 `json:"rank,omitempty"`
	Source      string              `json:"source,omitempty"`
	LocalV4     []string            `json:"local_v4_ips,omitempty"`
	LocalV6     []string            `json:"local_v6_ips,omitempty"`
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
			if strings.Contains(target, ":") {
				af = 6
			} else {
				af = 4
			}
			dns := atlas.Definition{
				Description:      "DNS A lookup for " + domain,
				Type:             "dns",
				AF:               af,
				IsOneoff:         true,
				QueryClass:       "IN",
				QueryType:        "A",
				Target:           target,
				QueryArgument:    domain,
				ResolveOnProbe:   true,
				UseProbeResolver: selfResolve,
				SetRDBit:         true,
			}
			ret = append(ret, dns)
			dns = atlas.Definition{
				Description:      "DNS AAAA lookup for " + domain,
				Type:             "dns",
				AF:               af,
				IsOneoff:         true,
				QueryClass:       "IN",
				QueryType:        "AAAA",
				Target:           target,
				QueryArgument:    domain,
				ResolveOnProbe:   true,
				UseProbeResolver: selfResolve,
				SetRDBit:         true,
			}
			ret = append(ret, dns)
		}
	}

	return ret
}

// LookupAtlas uses apiKey to do DNS (A and AAAA) lookups for domains from
// probeIds
func LookupAtlas(queries []string, apiKey string, probeIds []string, targets []string, startTime time.Time) ([]int, error) {
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
	dnsDefinitions := makeDNSDefinitions(queries, targets)

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
