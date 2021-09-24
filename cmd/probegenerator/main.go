package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	arg "github.com/alexflint/go-arg"
	atlas "github.com/keltia/ripe-atlas"
	probes "github.com/timartiny/RipeProbe/probes"
)

var (
	infoLogger    *log.Logger
	errorLogger   *log.Logger
	SKIPCOUNTRIES []string
)

type Probe atlas.Probe
type Probes []atlas.Probe

type ProbeGetterFlags struct {
	AllProbesPath      string `arg:"--all_probes_file" help:"Path to save all the probes data to"`
	FilteredProbesPath string `arg:"--filtered_probes_file,required" help:"(Required) Path to save the probes from not censored countryes, alive, and from different ASNs to"`
}

func contains(l []string, s string) bool {
	for _, a := range l {
		if a == s {
			return true
		}
	}

	return false
}

func getProbes() Probes {
	client, err := atlas.NewClient(atlas.Config{})
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}
	opts := make(map[string]string)
	opts["status"] = "1"
	opts["prefix_v4"] = "0.0.0.0/0"
	opts["prefix_v6"] = "0:0:0:0:0:0:0:0/0"
	probeSlice, err := client.GetProbes(opts)
	if err != nil {
		errorLogger.Fatalf("Error getting probes, err: %v\n", err)
	}
	return probeSlice
}

func getProbesNotCountry(allProbes Probes) Probes {
	var ret Probes
	infoLogger.Printf("Filtering out all probes that come from 'censored' " +
		"countries",
	)
	for _, probe := range allProbes {
		if !contains(SKIPCOUNTRIES, probe.CountryCode) {
			if len(probe.AddressV4) > 0 && len(probe.AddressV6) > 0 {
				ret = append(ret, probe)
			}
		}
	}

	return ret
}

func writeProbesToFile(path string, probeSlice Probes) {
	file, err := os.Create(path)
	if err != nil {
		errorLogger.Fatalf("Error creating file: %s, %v\n", path, err)
	}

	simpleProbeSlice := probes.AtlasProbeSliceToSimpleProbeSlice(probeSlice)

	for _, probe := range simpleProbeSlice {
		jsonBytes, err := json.Marshal(probe)
		if err != nil {
			errorLogger.Printf("Error converting probe to json: %v\n", err)
			continue
		}
		file.Write(jsonBytes)
		file.WriteString("\n")
	}
}

const (
	NeverConnected int = iota
	Connected
	Disconnected
	Abandoned
)

func filterAlive(probeSlice Probes) Probes {
	infoLogger.Printf("Filtering out probes that haven't been alive for a week")
	var ret Probes

	for _, probe := range probeSlice {
		if probe.Status.ID == Connected {
			connectedSince, err := time.Parse(time.RFC3339, probe.Status.Since)
			if err != nil {
				errorLogger.Panicf("time.Parse Error: %v\n", err)
			}
			if connectedSince.Before(time.Now().AddDate(0, 0, -7)) {
				ret = append(ret, probe)
			}
		}
	}

	return ret
}

func filterAS(probeSlice Probes) Probes {
	infoLogger.Printf("Filtering out probes from duplicate v4 and v6 ASNs")
	var ret Probes
	v4ASN := make(map[int]bool)
	v6ASN := make(map[int]bool)

	for _, probe := range probeSlice {
		v4OK := v4ASN[probe.AsnV4]
		v6OK := v6ASN[probe.AsnV6]
		if !v4OK && !v6OK {
			v4ASN[probe.AsnV4] = true
			v6ASN[probe.AsnV6] = true
			ret = append(ret, probe)
		}
	}

	return ret
}

func setupArgs() ProbeGetterFlags {
	var args ProbeGetterFlags
	arg.MustParse(&args)

	return args
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
	SKIPCOUNTRIES = []string{"CN", "IR", "RU", "SA", "KR", "IN", "PK", "EG", "AR", "BR"}

	args := setupArgs()

	infoLogger.Printf(
		"Getting all active probes with v4 and v6 addresses from RIPE Atlas, " +
			"this is the longest part, takes around a minute",
	)
	allProbes := getProbes()
	infoLogger.Printf("number of probes: %d\n", len(allProbes))
	if len(args.AllProbesPath) > 0 {
		infoLogger.Printf(
			"Saving the probes (simplified) data to %s\n", args.AllProbesPath,
		)
		writeProbesToFile(args.AllProbesPath, allProbes)
	}
	nonCensoredProbes := getProbesNotCountry(allProbes)
	infoLogger.Printf("number of noncensored probes: %d\n", len(nonCensoredProbes))
	aliveNonCensoredProbes := filterAlive(nonCensoredProbes)
	infoLogger.Printf("number of non-censored probes alive a week: %d\n", len(aliveNonCensoredProbes))
	nonDuplicateAS := filterAS(aliveNonCensoredProbes)
	infoLogger.Printf("Number of non-duplicate ASes probes: %d\n", len(nonDuplicateAS))
	writeProbesToFile(args.FilteredProbesPath, nonDuplicateAS)
}
