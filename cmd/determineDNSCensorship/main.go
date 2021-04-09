package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
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
type IPPair struct {
	V4 IPs
	V6 IPs
}
type DomainToIPs map[string]IPPair
type Resolvers struct {
	Domains DomainToIPs
	Open    IPPair
}

func (l IPs) Contains(n net.IP) bool {
	for _, v := range l {
		if v.Equal(n) {
			return true
		}
	}

	return false
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
			ip := net.ParseIP(qr.ResolverIP)
			if ip.To4() != nil {
				toDC[qr.ResolverType] = IPPair{
					V4: append(toDC[qr.ResolverType].V4, ip),
					V6: toDC[qr.ResolverType].V6,
				}
			} else {
				toDC[qr.ResolverType] = IPPair{
					V4: toDC[qr.ResolverType].V4,
					V6: append(toDC[qr.ResolverType].V6, ip),
				}

			}
		}
	}

	dC <- toDC
	rC <- toRC
}

func getResolverStats(fullResults Results) Resolvers {
	var ret Resolvers
	ret.Domains = make(DomainToIPs)
	domainChan := make(chan DomainToIPs)
	resolverChan := make(chan IPs)
	var resolverWg sync.WaitGroup

	go func(domainChan <-chan DomainToIPs) {
		for dtp := range domainChan {
			for k, v := range dtp {
				for _, ip := range v.V4 {
					if !ret.Domains[k].V4.Contains(ip) {
						ret.Domains[k] = IPPair{
							V4: append(ret.Domains[k].V4, ip),
							V6: ret.Domains[k].V6,
						}
					}
				}
				for _, ip := range v.V6 {
					if !ret.Domains[k].V6.Contains(ip) {
						ret.Domains[k] = IPPair{
							V4: ret.Domains[k].V4,
							V6: append(ret.Domains[k].V6, ip),
						}
					}
				}
			}
		}
	}(domainChan)

	go func(resolverChan <-chan IPs) {
		for ips := range resolverChan {
			for _, ip := range ips {
				if ip.To4() != nil {
					if !ret.Open.V4.Contains(ip) {
						ret.Open.V4 = append(ret.Open.V4, ip)
					}
				} else {
					if !ret.Open.V6.Contains(ip) {
						ret.Open.V6 = append(ret.Open.V6, ip)
					}
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
			&resolverWg)
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

	return ret
}

type Queries []string

func getQueryStats(fullResults Results) Queries {
	var ret Queries

	pr := fullResults[0]
	queries := pr.V4ToV4[0].Queries
	for k := range queries {
		ret = append(ret, k)
	}

	return ret
}

func printResolverStats(rs Resolvers, printIPs bool) {
	numDomainResolvers := len(rs.Domains)
	numDomainIPs := 0
	for _, ips := range rs.Domains {
		numDomainIPs += len(ips.V4)
		numDomainIPs += len(ips.V6)
	}
	numOpenResolvers := len(rs.Open.V4) + len(rs.Open.V6)
	numResolvers := numDomainResolvers + numOpenResolvers
	fmt.Printf("Number of Resolvers: %d\n", numResolvers)
	fmt.Printf(
		"\tNumber of Domains: %d (%d ips)\n",
		numDomainResolvers,
		numDomainIPs,
	)
	if printIPs {
		for d, ips := range rs.Domains {
			fmt.Printf("\t\t%s:\t%d ips\n", d, len(ips.V4)+len(ips.V6))
			fmt.Printf("\t\t\tV4 IPs: %d\n", len(ips.V4))
			for _, ip := range ips.V4 {
				fmt.Printf("\t\t\t\t%s\n", ip.String())
			}
			fmt.Printf("\t\t\tV6 Ips: %d\n", len(ips.V6))
			for _, ip := range ips.V6 {
				fmt.Printf("\t\t\t\t%s\n", ip.String())
			}
		}
	}
	fmt.Printf("\tNumber of Open Resolvers: %d\n", numOpenResolvers)
	if printIPs {
		fmt.Printf("\t\t\tV4 IPs: %d\n", len(rs.Open.V4))
		for _, ip := range rs.Open.V4 {
			fmt.Printf("\t\t\t\t%s\n", ip.String())
		}
		fmt.Printf("\t\t\tV6 IPs: %d\n", len(rs.Open.V6))
		for _, ip := range rs.Open.V6 {
			fmt.Printf("\t\t\t\t%s\n", ip.String())
		}
	}
}

func printQueryStats(qs Queries) {
	fmt.Printf("Domains to be resolved: %d\n", len(qs))
	for _, q := range qs {
		fmt.Printf("\t%s\n", q)
	}
}

func printGeneralStats(gS GenStats, printIPs bool) {
	fmt.Printf("Number of probes in this measurement: %d\n", gS.NumProbes)
	printResolverStats(gS.ResolverStats, printIPs)
	printQueryStats(gS.QueryStats)
}

func generalStats(fullResults Results, genChan chan<- GenStats) {
	var ret GenStats
	ret.NumProbes = getNumProbes(fullResults)
	ret.ResolverStats = getResolverStats(fullResults)
	ret.QueryStats = getQueryStats(fullResults)

	genChan <- ret
}

type GenStats struct {
	NumProbes     int
	ResolverStats Resolvers
	QueryStats    Queries
}

func printSpecificResults(stats SpecificResults) {
	for dom, spDom := range stats {
		fmt.Printf("\tfor %s:\n", dom)
		fmt.Printf("\t\tOpen Resolvers responded:\n")
		if spDom.Open.Addresses > 0 {
			fmt.Printf("\t\t\tWith Addresses: %d\n", spDom.Open.Addresses)
		}
		if spDom.Open.Timeouts > 0 {
			fmt.Printf("\t\t\tWith Timeouts: %d\n", spDom.Open.Timeouts)
		}
		if spDom.Open.NameServers > 0 {
			fmt.Printf("\t\t\tWith NameServers: %d\n", spDom.Open.NameServers)
		}
		if spDom.Open.Other > 0 {
			fmt.Printf("\t\t\tWith Other: %d\n", spDom.Open.Other)
			fmt.Printf("\t\t\t\tSee: \n")
			for _, in := range spDom.Domain.OtherInfo {
				fmt.Printf(
					"\t\t\t\t\t{ProbeID: %d, Target: %s}\n",
					in.ProbeID,
					in.IP.String(),
				)
			}
		}

		fmt.Printf("\t\tDomain IPs responded:\n")
		if spDom.Domain.Addresses > 0 {
			fmt.Printf("\t\t\tWith Addresses: %d\n", spDom.Domain.Addresses)
		}
		if spDom.Domain.Timeouts > 0 {
			fmt.Printf("\t\t\tWith Timeouts: %d\n", spDom.Domain.Timeouts)
		}
		if spDom.Domain.NameServers > 0 {
			fmt.Printf("\t\t\tWith NameServers: %d\n", spDom.Domain.NameServers)
		}
		if spDom.Domain.Other > 0 {
			fmt.Printf("\t\t\tWith Other: %d\n", spDom.Domain.Other)
			fmt.Printf("\t\t\t\tSee: \n")
			for _, in := range spDom.Domain.OtherInfo {
				fmt.Printf(
					"\t\t\t\t\t{ProbeID: %d, Target: %s}\n",
					in.ProbeID,
					in.IP.String(),
				)
			}
		}

	}
}

func printStats(genChan <-chan GenStats,
	v4AChan <-chan SpecificResults,
	v4AAAAChan <-chan SpecificResults,
	v6AChan <-chan SpecificResults,
	v6AAAAChan <-chan SpecificResults,
	wg *sync.WaitGroup,
	printIPs bool,
) {
	gS := <-genChan
	printGeneralStats(gS, printIPs)
	wg.Done()

	v4AStats := <-v4AChan
	fmt.Printf("When a V4 address was asked to resolve a domain for an A record:\n")
	printSpecificResults(v4AStats)
	wg.Done()

	v4AAAAStats := <-v4AAAAChan
	fmt.Printf("When a V4 address was asked to resolve a domain for an AAAA record:\n")
	printSpecificResults(v4AAAAStats)
	wg.Done()

	v6AStats := <-v6AChan
	fmt.Printf("When a V6 address was asked to resolve a domain for an A record:\n")
	printSpecificResults(v6AStats)
	wg.Done()

	v6AAAAStats := <-v6AAAAChan
	fmt.Printf("When a V6 address was asked to resolve a domain for an AAAA record:\n")
	printSpecificResults(v6AAAAStats)
	wg.Done()
}

type Info struct {
	ProbeID int
	IP      net.IP
}
type Infos []Info
type SpecificResponse struct {
	Addresses   int
	Timeouts    int
	NameServers int
	Other       int
	OtherInfo   Infos
}
type SpecificDomain struct {
	Open   SpecificResponse
	Domain SpecificResponse
}
type SpecificResults map[string]SpecificDomain

func update(sr, newSr SpecificResponse) SpecificResponse {
	sr.Addresses += newSr.Addresses
	sr.Timeouts += newSr.Timeouts
	sr.NameServers += newSr.NameServers
	sr.Other += newSr.Other
	sr.OtherInfo = append(sr.OtherInfo, newSr.OtherInfo...)
	return sr
}

func isUrl(str string) bool {
	// _, err := url.ParseRequestURI(str)
	// if err != nil {
	// 	return false
	// }
	_, err := url.Parse(str)
	return err == nil
}

func getSpecificResponse(strs []string, prID int, resIP string) SpecificResponse {
	info := Info{ProbeID: prID, IP: net.ParseIP(resIP)}
	var sr SpecificResponse
	for _, str := range strs {
		if net.ParseIP(str) != nil {
			sr.Addresses++
		} else if strings.Split(str, ": ")[0] == "timeout" {
			sr.Timeouts++
		} else if len(str) == 0 {
			infoLogger.Printf("len of str is 0\n")
		} else if isUrl(str) {
			sr.NameServers++
		} else {
			sr.Other++
			sr.OtherInfo = append(sr.OtherInfo, info)
		}
	}

	return sr
}

func v6AAAAStats(fR Results, v6AAAAChan chan<- SpecificResults) {
	toChan := make(SpecificResults)

	for _, pr := range fR {
		for _, qr := range pr.V6ToV6 {
			for dom, resps := range qr.Queries {
				if _, ok := toChan[dom]; !ok {
					toChan[dom] = SpecificDomain{
						Open:   SpecificResponse{},
						Domain: SpecificResponse{},
					}
				}
				if strings.Contains(qr.ResolverType, "_Resolver") {
					resp := getSpecificResponse(resps, pr.ProbeID, qr.ResolverIP)
					toChan[dom] = SpecificDomain{
						Open:   update(toChan[dom].Open, resp),
						Domain: toChan[dom].Domain,
					}
				} else {
					resp := getSpecificResponse(resps, pr.ProbeID, qr.ResolverIP)
					toChan[dom] = SpecificDomain{
						Open:   toChan[dom].Open,
						Domain: update(toChan[dom].Domain, resp),
					}
				}
			}

		}
	}

	v6AAAAChan <- toChan
}

func v4AAAAStats(fR Results, v4AAAAChan chan<- SpecificResults) {
	toChan := make(SpecificResults)

	for _, pr := range fR {
		for _, qr := range pr.V4ToV6 {
			for dom, resps := range qr.Queries {
				if _, ok := toChan[dom]; !ok {
					toChan[dom] = SpecificDomain{
						Open:   SpecificResponse{},
						Domain: SpecificResponse{},
					}
				}
				if strings.Contains(qr.ResolverType, "_Resolver") {
					resp := getSpecificResponse(resps, pr.ProbeID, qr.ResolverIP)
					toChan[dom] = SpecificDomain{
						Open:   update(toChan[dom].Open, resp),
						Domain: toChan[dom].Domain,
					}
				} else {
					resp := getSpecificResponse(resps, pr.ProbeID, qr.ResolverIP)
					toChan[dom] = SpecificDomain{
						Open:   toChan[dom].Open,
						Domain: update(toChan[dom].Domain, resp),
					}
				}
			}

		}
	}

	v4AAAAChan <- toChan
}

func v6AStats(fR Results, v6AChan chan<- SpecificResults) {
	toChan := make(SpecificResults)

	for _, pr := range fR {
		for _, qr := range pr.V6ToV4 {
			for dom, resps := range qr.Queries {
				if _, ok := toChan[dom]; !ok {
					toChan[dom] = SpecificDomain{
						Open:   SpecificResponse{},
						Domain: SpecificResponse{},
					}
				}
				if strings.Contains(qr.ResolverType, "_Resolver") {
					resp := getSpecificResponse(resps, pr.ProbeID, qr.ResolverIP)
					toChan[dom] = SpecificDomain{
						Open:   update(toChan[dom].Open, resp),
						Domain: toChan[dom].Domain,
					}
				} else {
					resp := getSpecificResponse(resps, pr.ProbeID, qr.ResolverIP)
					toChan[dom] = SpecificDomain{
						Open:   toChan[dom].Open,
						Domain: update(toChan[dom].Domain, resp),
					}
				}
			}

		}
	}

	v6AChan <- toChan
}

func v4AStats(fR Results, v4AChan chan<- SpecificResults) {
	toChan := make(SpecificResults)

	for _, pr := range fR {
		for _, qr := range pr.V4ToV4 {
			for dom, resps := range qr.Queries {
				if _, ok := toChan[dom]; !ok {
					toChan[dom] = SpecificDomain{
						Open:   SpecificResponse{},
						Domain: SpecificResponse{},
					}
				}
				if strings.Contains(qr.ResolverType, "_Resolver") {
					resp := getSpecificResponse(resps, pr.ProbeID, qr.ResolverIP)
					toChan[dom] = SpecificDomain{
						Open:   update(toChan[dom].Open, resp),
						Domain: toChan[dom].Domain,
					}
				} else {
					resp := getSpecificResponse(resps, pr.ProbeID, qr.ResolverIP)
					toChan[dom] = SpecificDomain{
						Open:   toChan[dom].Open,
						Domain: update(toChan[dom].Domain, resp),
					}
				}
			}

		}
	}

	v4AChan <- toChan
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
	genChan := make(chan GenStats)
	v4AChan := make(chan SpecificResults)
	v4AAAAChan := make(chan SpecificResults)
	v6AChan := make(chan SpecificResults)
	v6AAAAChan := make(chan SpecificResults)
	fullResults := getStruct(*resultsPath)

	go printStats(
		genChan, v4AChan, v4AAAAChan, v6AChan, v6AAAAChan, &wg, *printIPs,
	)

	wg.Add(1)
	go generalStats(fullResults, genChan)

	wg.Add(1)
	go v4AStats(fullResults, v4AChan)
	wg.Add(1)
	go v4AAAAStats(fullResults, v4AAAAChan)
	wg.Add(1)
	go v6AStats(fullResults, v6AChan)
	wg.Add(1)
	go v6AAAAStats(fullResults, v6AAAAChan)

	wg.Wait()
}
