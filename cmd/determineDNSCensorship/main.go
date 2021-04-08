package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"sync"

	results "github.com/timartiny/RipeProbe/results"
)

var infoLogger *log.Logger
var errorLogger *log.Logger

type Results []results.ProbeResult

func getStruct(path string) Results {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", path, err)
	}
	defer file.Close()
	b, err := ioutil.ReadAll(file)
	if err != nil {
		errorLogger.Fatalf("Error reading file, %v\n", err)
	}

	var res Results

	err = json.Unmarshal(b, &res)
	if err != nil {
		errorLogger.Fatalf("Error unmarshaling data, %v\n", err)
	}

	return res
}

func getNumProbes(r Results) int {
	return len(r)
}

type Ints []int

func (a Ints) Len() int           { return len(a) }
func (a Ints) Less(i, j int) bool { return a[i] < a[j] }
func (a Ints) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func getProbes(r Results) Ints {
	var ret Ints
	for _, t := range r {
		ret = append(ret, t.ProbeID)
	}
	sort.Sort(ret)

	return ret
}

type IPs []net.IP
type DomainToIPs map[string]IPs

func (l IPs) Contains(n net.IP) bool {
	for _, v := range l {
		if v.Equal(n) {
			return true
		}
	}

	return false
}

type ResolverStats struct {
	Domains       DomainToIPs
	OpenResolvers IPs
}

func resolverStatsFromProbe(
	qrs []results.QueryResult,
	dC chan<- DomainToIPs,
	rC chan<- IPs,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	toDC := make(DomainToIPs)
	var toRC IPs

	for _, qr := range qrs {
		if strings.Contains(qr.ResolverType, "_Resolver") {
			toRC = append(toRC, net.ParseIP(qr.ResolverIP))
		} else {
			if len(qr.ResolverType) <= 0 {
				fmt.Printf("empty one: %s: %v\n", qr.ResolverType, qr.ResolverIP)
				continue
			}
			toDC[qr.ResolverType] = append(
				toDC[qr.ResolverType],
				net.ParseIP(qr.ResolverIP),
			)
		}
	}

	dC <- toDC
	rC <- toRC
}

func getResolverStats(fullResults Results) ResolverStats {
	var ret ResolverStats
	ret.Domains = make(DomainToIPs)
	domainChan := make(chan DomainToIPs)
	resolverChan := make(chan IPs)
	var resolverWg sync.WaitGroup
	domainIpCount := make(map[string]int)
	resolverIpCount := make(map[string]int)

	go func(domainChan <-chan DomainToIPs) {
		for dtp := range domainChan {
			for k, v := range dtp {
				for _, ip := range v {
					domainIpCount[ip.String()]++
					if !ret.Domains[k].Contains(ip) {
						ret.Domains[k] = append(ret.Domains[k], ip)
					}
				}
			}
		}
	}(domainChan)

	go func(resolverChan <-chan IPs) {
		for ips := range resolverChan {
			for _, ip := range ips {
				resolverIpCount[ip.String()]++
				if !ret.OpenResolvers.Contains(ip) {
					ret.OpenResolvers = append(ret.OpenResolvers, ip)
				}

			}
		}
	}(resolverChan)

	for _, probeResults := range fullResults {
		resolverWg.Add(4)
		go resolverStatsFromProbe(
			probeResults.V4ToV4,
			domainChan,
			resolverChan,
			&resolverWg,
		)
		go resolverStatsFromProbe(
			probeResults.V4ToV6,
			domainChan,
			resolverChan,
			&resolverWg,
		)
		go resolverStatsFromProbe(
			probeResults.V6ToV4,
			domainChan,
			resolverChan,
			&resolverWg,
		)
		go resolverStatsFromProbe(
			probeResults.V6ToV6,
			domainChan,
			resolverChan,
			&resolverWg,
		)
	}

	resolverWg.Wait()

	close(domainChan)
	close(resolverChan)
	infoLogger.Printf("domain ipCount: %v\n", domainIpCount)
	infoLogger.Printf("resolver ipCount: %v\n", resolverIpCount)

	return ret
}

type QueryStats []string

func getQueryStats(fullResults Results) QueryStats {
	var ret QueryStats

	pr := fullResults[0]
	queries := pr.V4ToV4[0].Queries
	for k := range queries {
		ret = append(ret, k)
	}

	return ret
}

func printResolverStats(rs ResolverStats, printIPs bool) {
	numDomainResolvers := len(rs.Domains)
	numDomainIPs := 0
	for _, ips := range rs.Domains {
		numDomainIPs += len(ips)
	}
	numOpenResolvers := len(rs.OpenResolvers)
	numResolvers := numDomainResolvers + numOpenResolvers
	fmt.Printf("Number of Resolvers: %d\n", numResolvers)
	fmt.Printf(
		"\tNumber of Domains: %d (%d ips)\n",
		numDomainResolvers,
		numDomainIPs,
	)
	for d, ips := range rs.Domains {
		var v4s []net.IP
		var v6s []net.IP
		for _, ip := range ips {
			if ip.To4() != nil {
				v4s = append(v4s, ip)
			} else {
				v6s = append(v6s, ip)
			}
		}
		if printIPs {
			fmt.Printf("\t\t%s:\t%d ips\n", d, len(ips))
			fmt.Printf("\t\t\tV4 IPs: %d\n", len(v4s))
			for _, ip := range v4s {
				fmt.Printf("\t\t\t\t%s\n", ip.String())
			}
			fmt.Printf("\t\t\tV6 Ips: %d\n", len(v6s))
			for _, ip := range v6s {
				fmt.Printf("\t\t\t\t%s\n", ip.String())
			}
		}
	}
	fmt.Printf("\tNumber of Open Resolvers: %d\n", numOpenResolvers)
	var openV4s []net.IP
	var openV6s []net.IP
	for _, ip := range rs.OpenResolvers {
		if ip.To4() != nil {
			openV4s = append(openV4s, ip)
		} else {
			openV6s = append(openV6s, ip)
		}
	}
	if printIPs {
		fmt.Printf("\t\t\tV4 IPs: %d\n", len(openV4s))
		for _, ip := range openV4s {
			fmt.Printf("\t\t\t\t%s\n", ip.String())
		}
		fmt.Printf("\t\t\tV6 IPs: %d\n", len(openV6s))
		for _, ip := range openV6s {
			fmt.Printf("\t\t\t\t%s\n", ip.String())
		}
	}
}

func printQueryStats(qs QueryStats) {
	fmt.Printf("Domains to be resolved: %d\n", len(qs))
	for _, q := range qs {
		fmt.Printf("\t%s\n", q)
	}
}

func printGeneralStats(
	np int,
	rs ResolverStats,
	printIPs bool,
	qs QueryStats,
) {
	fmt.Printf("Number of probes in this measurement: %d\n", np)
	printResolverStats(rs, printIPs)
	printQueryStats(qs)
}

func generalStats(fullResults Results, printIPs bool, wg *sync.WaitGroup) {
	defer wg.Done()
	numProbes := getNumProbes(fullResults)
	resolverStats := getResolverStats(fullResults)
	queryStats := getQueryStats(fullResults)

	printGeneralStats(numProbes, resolverStats, printIPs, queryStats)
}

func main() {
	resultsPath := flag.String("r", "", "Path to results file")
	printIPs := flag.Bool("ips", false, "Determine whether to print IPs of resolvers")
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
	var wg sync.WaitGroup
	fullResults := getStruct(*resultsPath)

	wg.Add(1)
	go generalStats(fullResults, *printIPs, &wg)

	wg.Wait()

}
