package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

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

func isUrl(str string) bool {
	// this is almost certainly a bad way to do it:

	return strings.ContainsAny(str, ".-")
}

type ResolverResults []*ResolverResult

// ResolverResult stores the ip address of a given resolver, the type of
// resolver it is (either the string "open", or the domain name of the resolver)
// and then a slice of domain results, how that resolver responded to
// queries for domains
type ResolverResult struct {
	ResolverIP    net.IP
	ResolverType  string
	DomainResults []*DomainResult
}

// Domain Result stores the domain name that was resolved, what the actual
// A record request results were (ips, and NSs) as well as AAAA, and whether
// any of those results successfully load the given domain page.
// We will require that all of the A int results add up to the number of probes
// (same for AAAA)
type DomainResult struct {
	Domain            string
	AResponse         *DNSResponse
	ASuccessesIP      int
	ASuccessProbes    []int
	AFailedIP         int
	ASuccessesNS      int
	AFailedNS         int
	ATimeouts         int
	AAuthority        int
	AAAAResponse      *DNSResponse
	AAAASuccessesIP   int
	AAAASuccessProbes []int
	AAAAFailedIP      int
	AAAASuccessesNS   int
	AAAAFailedNS      int
	AAAATimeouts      int
	AAAAAuthority     int
}

// DNSResponse is the actual response to DNS queries, the list of IPs, Nameservers
// or number of timeouts, and then a catch-all of other issues.
type DNSResponse struct {
	IPs       []net.IP
	NSs       []string
	Timeouts  bool
	Authority bool
	Others    []string
}

func strContains(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}

	return false
}

func ipContains(arr []net.IP, ip net.IP) bool {
	for _, i := range arr {
		if i.Equal(ip) {
			return true
		}
	}

	return false
}

func (dnsr *DNSResponse) Append(newDNSR *DNSResponse) *DNSResponse {
	if dnsr == nil {
		return newDNSR
	}
	if newDNSR == nil {
		return dnsr
	}
	for _, ip := range newDNSR.IPs {
		if !ipContains(dnsr.IPs, ip) {
			dnsr.IPs = append(dnsr.IPs, ip)
		}
	}

	for _, ns := range newDNSR.NSs {
		if !strContains(dnsr.NSs, ns) {
			dnsr.NSs = append(dnsr.NSs, ns)
		}
	}

	dnsr.Timeouts = newDNSR.Timeouts || dnsr.Timeouts
	dnsr.Authority = newDNSR.Authority || dnsr.Authority

	for _, other := range newDNSR.Others {
		if !strContains(dnsr.Others, other) {
			dnsr.Others = append(dnsr.Others, other)
		}
	}

	return dnsr
}

type IPStatus int

const (
	Unknown IPStatus = iota
	Success
	Failure
)

var quickCheckConnDetailsChan chan ConnDetails
var quickCheckStatusChan chan IPStatus

func quickIPDomainLookup() {
	var quickLookupMap = map[string]map[string]IPStatus{} // [domain, ip] -> status

	for cd := range quickCheckConnDetailsChan {
		if cd.Status != Unknown {
			if quickLookupMap[cd.Domain] == nil {
				quickLookupMap[cd.Domain] = map[string]IPStatus{}
			}
			quickLookupMap[cd.Domain][cd.IP.String()] = cd.Status
		} else {
			if quickLookupMap[cd.Domain] == nil {
				quickCheckStatusChan <- Unknown
			} else {
				quickCheckStatusChan <- quickLookupMap[cd.Domain][cd.IP.String()]
			}
		}
	}
}

type ConnDetails struct {
	Domain string
	IP     net.IP
	Status IPStatus
}

func ipChecker(connDetailsChan <-chan ConnDetails, isValidChan chan<- bool) {
	for cd := range connDetailsChan {
		quickCheckConnDetailsChan <- cd
		status := <-quickCheckStatusChan
		if status != Unknown {
			isValidChan <- status == Success
			continue
		}
		dialer := &net.Dialer{
			Timeout: time.Second,
		}
		config := &tls.Config{
			ServerName: cd.Domain,
		}

		conn, err := tls.DialWithDialer(
			dialer, "tcp", net.JoinHostPort(cd.IP.String(), "443"), config,
		)
		if err != nil {
			cd.Status = Failure
			quickCheckConnDetailsChan <- cd
			isValidChan <- false
			continue
		}

		err = conn.VerifyHostname(cd.Domain)
		toSend := err == nil
		if toSend {
			cd.Status = Success
		} else {
			cd.Status = Failure
		}
		quickCheckConnDetailsChan <- cd
		isValidChan <- toSend
	}

}

func ipSuccess(dnsr *DNSResponse, domain string) bool {
	var ret bool

	const numIPCheckers = 10
	connDetailsChan := make(chan ConnDetails, len(dnsr.IPs))
	resultsChan := make(chan bool, len(dnsr.IPs))

	for i := 0; i < numIPCheckers; i++ {
		go ipChecker(connDetailsChan, resultsChan)
	}

	for _, ip := range dnsr.IPs {
		connDetailsChan <- ConnDetails{Domain: domain, IP: ip}
	}
	close(connDetailsChan)
	for i := 0; i < len(dnsr.IPs); i++ {
		ret = <-resultsChan
		if ret {
			// infoLogger.Printf("Got true, for %s!\n", domain)
			break
		}
	}

	return ret
}

func nsSuccess(dnsr *DNSResponse, domain string) bool {
	var ret bool
	if len(dnsr.NSs) == 0 {
		return ret
	}
	for _, ns := range dnsr.NSs {
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Second,
				}
				return d.DialContext(ctx, network, net.JoinHostPort(ns, "53"))
			},
		}
		ips, err := r.LookupHost(context.Background(), domain)
		if err != nil || len(ips) == 0 {
			return ret
		}

		tmpDNSR := new(DNSResponse)
		for _, ip := range ips {
			tmpDNSR.IPs = append(tmpDNSR.IPs, net.ParseIP(ip))
		}
		if ipSuccess(tmpDNSR, domain) {
			ret = true
			break
		}
	}

	return ret
}

func parseQueryResults(
	qrc <-chan QueryResultsAndNumAs,
	consolidateChan chan<- ResolverResults,
	wg *sync.WaitGroup,
) {
	for qra := range qrc {
		numAs := qra.NumAs
		qrs := qra.QueryResults
		var ret ResolverResults
		for _, qr := range qrs {
			rr := new(ResolverResult)
			rr.ResolverIP = net.ParseIP(qr.ResolverIP)
			if strings.Contains(qr.ResolverType, "_Resolver") {
				rr.ResolverType = "Open Resolver"
			} else {
				rr.ResolverType = qr.ResolverType
			}
			for domain, responses := range qr.Queries {
				dr := new(DomainResult)
				dnsr := new(DNSResponse)
				dr.Domain = domain
				for _, response := range responses {
					if x := net.ParseIP(response); x != nil {
						dnsr.IPs = append(dnsr.IPs, x)
					} else if strings.Split(response, ": ")[0] == "timeout" {
						dnsr.Timeouts = true
					} else if len(response) == 0 {
						infoLogger.Printf("len of str is 0\n")
					} else if strings.Contains(response, "Authority") {
						dnsr.Authority = true
					} else if isUrl(response) {
						dnsr.NSs = append(dnsr.NSs, response)
					} else {
						dnsr.Others = append(dnsr.Others, response)
					}
				}
				if numAs == 1 {
					switch {
					case ipSuccess(dnsr, domain):
						dr.ASuccessesIP = 1
						dr.ASuccessProbes = append(
							dr.ASuccessProbes, qra.ProbeID,
						)
					case len(dnsr.IPs) > 0:
						dr.AFailedIP = 1
					case nsSuccess(dnsr, domain):
						dr.ASuccessesNS = 1
					case len(dnsr.NSs) > 0:
						dr.AFailedNS = 1
					case dnsr.Timeouts:
						dr.ATimeouts = 1
					case dnsr.Authority:
						dr.AAuthority = 1
					default:
						infoLogger.Printf("nothing got 1 this time...\n")
					}
					dr.AResponse = dnsr
					// pass dnsr, domain to check certs here and set ASuccessesIP, ASuccessesNS
				} else if numAs == 4 {
					switch {
					case ipSuccess(dnsr, domain):
						dr.AAAASuccessesIP = 1
						dr.AAAASuccessProbes = append(
							dr.AAAASuccessProbes, qra.ProbeID,
						)
					case len(dnsr.IPs) > 0:
						dr.AAAAFailedIP = 1
					case nsSuccess(dnsr, domain):
						dr.AAAASuccessesNS = 1
					case len(dnsr.NSs) > 0:
						dr.AAAAFailedNS = 1
					case dnsr.Timeouts:
						dr.AAAATimeouts = 1
					case dnsr.Authority:
						dr.AAAAAuthority = 1
					default:
						infoLogger.Printf("nothing got 1 this time...\n")
					}
					dr.AAAAResponse = dnsr
					// pass dnsr, domain to check certs here and set AAAASuccessesIP, AAAASuccessesNS
				}

				rr.DomainResults = append(rr.DomainResults, dr)
			}

			ret = append(ret, rr)
		}
		consolidateChan <- ret
		wg.Done()
	}
}

type QueryResultsAndNumAs struct {
	ProbeID      int
	QueryResults []results.QueryResult
	NumAs        int
}

func parseProbeResult(
	pr results.ProbeResult,
	rrChan chan<- ResolverResults,
	ctrChan chan<- int,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	queryResultChan := make(chan QueryResultsAndNumAs)
	defer close(queryResultChan)

	var tmpWg sync.WaitGroup
	go parseQueryResults(queryResultChan, rrChan, &tmpWg)
	ctrChan <- pr.ProbeID
	tmpWg.Add(4)
	queryResultChan <- QueryResultsAndNumAs{
		ProbeID: pr.ProbeID, QueryResults: pr.V4ToV4, NumAs: 1,
	}
	queryResultChan <- QueryResultsAndNumAs{
		ProbeID: pr.ProbeID, QueryResults: pr.V4ToV6, NumAs: 4,
	}
	queryResultChan <- QueryResultsAndNumAs{
		ProbeID: pr.ProbeID, QueryResults: pr.V6ToV4, NumAs: 1,
	}
	queryResultChan <- QueryResultsAndNumAs{
		ProbeID: pr.ProbeID, QueryResults: pr.V6ToV6, NumAs: 4,
	}
	tmpWg.Wait()
	ctrChan <- pr.ProbeID * -1
}

func consolidate(
	rrChan <-chan ResolverResults,
	mapChan chan<- map[string]*ResolverResult,
) {
	consolidated := make(map[string]*ResolverResult)

	for rrs := range rrChan {
		for _, rr := range rrs {
			if existingRR, ok := consolidated[rr.ResolverIP.String()]; !ok {
				consolidated[rr.ResolverIP.String()] = rr
			} else {
				for _, dr := range rr.DomainResults {
					for _, existingDR := range existingRR.DomainResults {
						if existingDR.Domain != dr.Domain {
							continue
						}
						existingDR.AResponse = existingDR.AResponse.Append(dr.AResponse)
						existingDR.AAAAResponse = existingDR.AAAAResponse.Append(dr.AResponse)
						existingDR.ASuccessesIP += dr.ASuccessesIP
						existingDR.ASuccessProbes = append(
							existingDR.ASuccessProbes, dr.ASuccessProbes...,
						)
						existingDR.AFailedIP += dr.AFailedIP
						existingDR.ASuccessesNS += dr.ASuccessesNS
						existingDR.AFailedNS += dr.AFailedNS
						existingDR.ATimeouts += dr.ATimeouts
						existingDR.AAuthority += dr.AAuthority
						existingDR.AAAASuccessesIP += dr.AAAASuccessesIP
						existingDR.AAAASuccessProbes = append(
							existingDR.AAAASuccessProbes,
							dr.AAAASuccessProbes...,
						)
						existingDR.AAAAFailedIP += dr.AAAAFailedIP
						existingDR.AAAASuccessesNS += dr.AAAASuccessesNS
						existingDR.AAAAFailedNS += dr.AAAAFailedNS
						existingDR.AAAATimeouts += dr.AAAATimeouts
						existingDR.AAAAAuthority += dr.AAAAAuthority
					}
				}
			}
		}
	}

	mapChan <- consolidated
}

func printIPResults(failures, successes int, succesProbes []int) {
	if successes > 0 {
		fmt.Printf("\t\t%d probe(s) received a valid IP", successes)
		fmt.Printf(" (Probe ids: %v)\n", succesProbes)
	}
	if failures > 0 {
		fmt.Printf("\t\t%d probe(s) received only invalid IPs\n", failures)
	}
}

func printTimeouts(num int) {
	if num > 0 {
		fmt.Printf("\t\t%d probe(s) timed out\n", num)
	}
}

func printAuthoritys(num int) {
	if num > 0 {
		fmt.Printf(
			"\t\t%d probe(s) received No Answer or Authority Given\n", num,
		)
	}
}

func printNSResults(failures, successes int) {
	if successes > 0 {
		fmt.Printf(
			"\t\t%d probe(s) received NameServers that served (locally) "+
				"valid IPs\n",
			successes,
		)
	}
	if failures > 0 {
		fmt.Printf(
			"\t\t%d probe(s) received NameServers that served (locally) "+
				"invalid IPs\n",
			failures,
		)
	}
}

func printResults(numProbes int, mapChan <-chan map[string]*ResolverResult) {

	m := <-mapChan
	fmt.Printf(
		"%d Probes were asked to use %d IPs as resolvers\n", numProbes, len(m),
	)

	for resIP, rr := range m {
		fmt.Printf("%s (%s)\n", resIP, rr.ResolverType)
		for _, domRes := range rr.DomainResults {
			fmt.Printf("\tFor A record requests for %s:\n", domRes.Domain)
			dnsRes := domRes.AResponse
			printIPResults(
				domRes.AFailedIP, domRes.ASuccessesIP, domRes.ASuccessProbes,
			)
			printNSResults(domRes.AFailedNS, domRes.ASuccessesNS)
			printTimeouts(domRes.ATimeouts)
			printAuthoritys(domRes.AAuthority)
			if len(dnsRes.Others) > 0 {
				fmt.Printf("\t\t%d probes received something else...\n", len(dnsRes.Others))
			}

			fmt.Printf("\tFor AAAA record requests for %s:\n", domRes.Domain)
			dnsRes = domRes.AAAAResponse
			printIPResults(
				domRes.AAAAFailedIP,
				domRes.AAAASuccessesIP,
				domRes.AAAASuccessProbes,
			)
			printNSResults(domRes.AAAAFailedNS, domRes.AAAASuccessesNS)
			printTimeouts(domRes.AAAATimeouts)
			printAuthoritys(domRes.AAAAAuthority)
			if len(dnsRes.Others) > 0 {
				fmt.Printf("\t\t%d probes received something else...\n", len(dnsRes.Others))
			}
		}
	}
}

func ctr(ctrChan <-chan int) {
	ctrMap := make(map[int]bool)
	for prbID := range ctrChan {
		if prbID > 0 {
			ctrMap[prbID] = true
		} else {
			delete(ctrMap, prbID*-1)
		}
		if len(ctrMap)%10 == 0 {
			infoLogger.Printf("%d probe lookups still running\n", len(ctrMap))
		}
	}
}

func main() {
	resultsPath := flag.String("r", "", "Path to results file")
	// printIPs := flag.Bool("ips", false, "Determine whether to print IPs of resolvers")
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

	rrChan := make(chan ResolverResults)
	mapChan := make(chan map[string]*ResolverResult)
	ctrChan := make(chan int)
	quickCheckConnDetailsChan = make(chan ConnDetails)
	quickCheckStatusChan = make(chan IPStatus)

	go quickIPDomainLookup()
	go consolidate(rrChan, mapChan)
	go ctr(ctrChan)
	for _, probeResult := range fullResults {
		wg.Add(1)
		go parseProbeResult(probeResult, rrChan, ctrChan, &wg)
	}
	// genChan := make(chan GenStats)
	// v4AChan := make(chan SpecificResults)
	// v4AAAAChan := make(chan SpecificResults)
	// v6AChan := make(chan SpecificResults)
	// v6AAAAChan := make(chan SpecificResults)

	// go printStats(
	// 	genChan, v4AChan, v4AAAAChan, v6AChan, v6AAAAChan, &wg, *printIPs,
	// )

	// wg.Add(1)
	// go generalStats(fullResults, genChan)

	// wg.Add(1)
	// go v4AStats(fullResults, v4AChan)
	// wg.Add(1)
	// go v4AAAAStats(fullResults, v4AAAAChan)
	// wg.Add(1)
	// go v6AStats(fullResults, v6AChan)
	// wg.Add(1)
	// go v6AAAAStats(fullResults, v6AAAAChan)

	wg.Wait()
	close(ctrChan)
	close(rrChan)
	printResults(len(fullResults), mapChan)
}
