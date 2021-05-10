package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	results "github.com/timartiny/RipeProbe/results"
)

var infoLogger *log.Logger
var errorLogger *log.Logger

type DomainToIPList map[string][]string

func (i DomainToIPList) String() string {
	var ret string
	ret = "{"
	total := 0
	for _, v := range i {
		total += len(v)
	}
	ret += fmt.Sprintf("%d", total)

	ret += "}"
	return ret
}

type Event struct {
	ValidIP   int
	InvalidIP int
	Timeout   int
	NoAns     int
	ValidNS   int
	InvalidNS int
	Data      DomainToIPList
}

func (e *Event) Update(otherE *Event) {
	e.ValidIP += otherE.ValidIP
	e.InvalidIP += otherE.InvalidIP
	e.Timeout += otherE.Timeout
	e.NoAns += otherE.NoAns
	e.ValidNS += otherE.ValidNS
	e.InvalidNS += otherE.InvalidNS

	for key, values := range otherE.Data {
		if _, ok := e.Data[key]; !ok {
			e.Data[key] = values
		} else {
			e.Data[key] = mergeSlices(values, e.Data[key])
		}
	}
}

func (e Event) String() string {
	var ret string
	ret += fmt.Sprintf("\t\tValidIP: %d\n", e.ValidIP)
	ret += fmt.Sprintf("\t\tInvalidIP: %d\n", e.InvalidIP)
	ret += fmt.Sprintf("\t\tTimeout: %d\n", e.Timeout)
	ret += fmt.Sprintf("\t\tNo Answer or Authority Given: %d\n", e.NoAns)
	ret += fmt.Sprintf("\t\tValidNS: %d\n", e.ValidNS)
	ret += fmt.Sprintf("\t\tInvalidNS: %d\n", e.InvalidNS)
	ret += fmt.Sprintf("\t\tData: %v", e.Data)
	return ret
}

func mergeSlices(v1, v2 []string) []string {
	var ret []string
	tracker := map[string]bool{}
	for _, s := range v1 {
		ret = append(ret, s)
		tracker[s] = true
	}

	for _, s := range v2 {
		if _, ok := tracker[s]; !ok {
			ret = append(ret, s)
		}
	}

	return ret
}

type ProbeResults []results.ProbeResult
type QueryResults []results.QueryResult
type Queries map[string][]string

type Single map[string]*Event // t['v4']= &Event{}

func (s Single) String() string {
	var ret string
	for d, e := range s {
		ret += fmt.Sprintf("\tAF %s:\n%v\n", d, *e)
	}

	return ret
}

func (s Single) Merge(otherS Single) {
	for af, eventPtr := range otherS {
		if _, ok := s[af]; !ok {
			s[af] = eventPtr
		} else {
			s[af].Update(eventPtr)
		}
	}
}

type Pair map[string]Single // t['resolver']['v4']= &Event{}

func (p Pair) String() string {
	var ret string
	for k, v := range p {
		ret += fmt.Sprintf("Resolver %s:\n%+v", k, v)
	}

	return ret
}

func (p Pair) Merge(otherP Pair) {
	for resolver, otherSingle := range otherP {
		if _, ok := p[resolver]; !ok {
			p[resolver] = otherSingle
		} else {
			p[resolver].Merge(otherSingle)
		}
	}
}

type Triplet map[string]Pair // t['probe']['resolver']['v4']= &Event{}

func (t Triplet) String() string {
	var ret string
	for k, v := range t {
		ret += fmt.Sprintf("Probe %s:\n%+v", k, v)
	}

	return ret
}

func getStruct(path string) ProbeResults {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Error opening file: %s, %v\n", path, err)
	}
	defer file.Close()
	b, err := ioutil.ReadAll(file)
	if err != nil {
		errorLogger.Fatalf("Error reading file, %v\n", err)
	}

	var res ProbeResults

	err = json.Unmarshal(b, &res)
	if err != nil {
		errorLogger.Fatalf("Error unmarshaling data, %v\n", err)
	}

	return res
}

func getEvent(domain string, answers []string, dataChan chan<- string) *Event {
	e := new(Event)
	e.Data = map[string][]string{}
	for _, answer := range answers {
		if strings.Contains(answer, "timeout") {
			e.Timeout++
		} else if strings.Contains(answer, "Authority") {
			e.NoAns++
		} else {
			e.Data[domain] = append(e.Data[domain], answer)
			dataChan <- answer
		}
	}

	return e
}

func contains(arr []string, s string) bool {
	for _, sa := range arr {
		if s == sa {
			return true
		}
	}

	return false
}

func queriesToSingle(queries Queries, vType string, uncensoredDomains []string, dc chan<- string) (Single, Single) {
	uncensoredSingle := Single{}
	uncensoredSingle[vType] = new(Event)
	uncensoredSingle[vType].Data = make(DomainToIPList)
	single := Single{}
	single[vType] = new(Event)
	single[vType].Data = make(DomainToIPList)

	for domain, answers := range queries {
		tEvent := getEvent(domain, answers, dc)
		if contains(uncensoredDomains, domain) {
			uncensoredSingle[vType].Update(tEvent)
		} else {
			single[vType].Update(tEvent)
		}
	}

	return single, uncensoredSingle
}

func queryResultsToPair(qResults QueryResults, vType string, uncensoredDomains []string, dc chan<- string) (Pair, Pair, Pair, Pair) {
	uncensoredPair := Pair{}
	uncensoredOpenPair := Pair{}
	pair := Pair{}
	openPair := Pair{}

	for _, qResult := range qResults {
		rIP := qResult.ResolverIP
		if strings.Contains(qResult.ResolverType, "Resolver") {
			openPair[rIP], uncensoredOpenPair[rIP] = queriesToSingle(qResult.Queries, vType, uncensoredDomains, dc)
		} else {
			pair[rIP], uncensoredPair[rIP] = queriesToSingle(qResult.Queries, vType, uncensoredDomains, dc)
		}
	}

	return pair, openPair, uncensoredPair, uncensoredOpenPair
}

func resultsToTriplets(pResults ProbeResults, uncensoredDomains []string, dc chan<- string) (Triplet, Triplet, Triplet, Triplet) {
	uncensoredTrip := Triplet{}
	uncensoredOpenTrip := Triplet{}
	trip := Triplet{}
	openTrip := Triplet{}
	for _, pResult := range pResults {
		pID := fmt.Sprintf("%d", pResult.ProbeID)
		if _, ok := trip[pID]; ok {
			infoLogger.Printf(
				"Probe id %d shows up more than once\n", pResult.ProbeID,
			)
		}
		trip[pID], openTrip[pID], uncensoredTrip[pID], uncensoredOpenTrip[pID] =
			queryResultsToPair(pResult.V4ToV4, "v4", uncensoredDomains, dc)
		tPair, tOpenPair, tuPair, tuOpenPair := queryResultsToPair(
			pResult.V4ToV6, "v6", uncensoredDomains, dc,
		)
		trip[pID].Merge(tPair)
		openTrip[pID].Merge(tOpenPair)
		uncensoredTrip[pID].Merge(tuPair)
		uncensoredOpenTrip[pID].Merge(tuOpenPair)
		tPair, tOpenPair, tuPair, tuOpenPair = queryResultsToPair(
			pResult.V6ToV4, "v4", uncensoredDomains, dc,
		)
		trip[pID].Merge(tPair)
		openTrip[pID].Merge(tOpenPair)
		uncensoredTrip[pID].Merge(tuPair)
		uncensoredOpenTrip[pID].Merge(tuOpenPair)
		tPair, tOpenPair, tuPair, tuOpenPair = queryResultsToPair(
			pResult.V6ToV6, "v6", uncensoredDomains, dc,
		)
		trip[pID].Merge(tPair)
		openTrip[pID].Merge(tOpenPair)
		uncensoredTrip[pID].Merge(tuPair)
		uncensoredOpenTrip[pID].Merge(tuOpenPair)
	}

	// infoLogger.Println(trip)
	return trip, openTrip, uncensoredTrip, uncensoredOpenTrip
}

type IPandCert struct {
	IP   string
	Cert *x509.Certificate
}
type IPCertMap map[string]*x509.Certificate

func lookupIP(
	ip string,
	ipCertChan chan<- *IPandCert,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	config := tls.Config{ServerName: "fake.com", InsecureSkipVerify: true}
	timeout := time.Duration(90) * time.Second
	dialConn, err := net.DialTimeout(
		"tcp", net.JoinHostPort(ip, "443"), timeout,
	)
	if err != nil {
		// errorLogger.Printf("net.DialTimeout error: %+v\n", err)
		return
	}

	tlsConn := tls.Client(dialConn, &config)
	defer tlsConn.Close()

	dialConn.SetReadDeadline(time.Now().Add(timeout))
	err = tlsConn.Handshake()
	if err != nil {
		// errorLogger.Printf("tlsConn.Handshake error: %+v\n", err)
		return
	}
	iac := new(IPandCert)
	iac.IP = ip
	iac.Cert = tlsConn.ConnectionState().PeerCertificates[0]
	ipCertChan <- iac
}

func checkData(
	dataInChan <-chan string,
	ipCertChan chan<- *IPandCert,
	wg *sync.WaitGroup,
) {
	checkMap := make(map[string]bool)
	total := 0

	for data := range dataInChan {
		if _, ok := checkMap[data]; !ok {
			checkMap[data] = true
			if ip := net.ParseIP(data); ip != nil {
				wg.Add(1)
				total += 1
				go lookupIP(data, ipCertChan, wg)
			}
		}
	}
	infoLogger.Printf("dataInChan closed, saw %v unique IPs\n", total)
}

func collectIPResults(
	ipCertChan <-chan *IPandCert, ipCertMapChan chan<- IPCertMap,
) {
	icm := make(IPCertMap)
	for x := range ipCertChan {
		if _, ok := icm[x.IP]; ok {
			errorLogger.Printf("Should never be here, we've seen an IP twice\n")
		} else {
			icm[x.IP] = x.Cert
		}
	}

	infoLogger.Printf("ipCertChan Closed\n")
	ipCertMapChan <- icm
}

func counter(countChan <-chan string) {
	openIPsMap := make(map[string]bool)
	for c := range countChan {
		if _, ok := openIPsMap[c]; !ok {
			openIPsMap[c] = true
		} else {
			delete(openIPsMap, c)
		}
		infoLogger.Printf("Waiting on %d tls lookups\n", len(openIPsMap))
		if len(openIPsMap) <= 10 {
			infoLogger.Printf("still open IPs: %v\n", openIPsMap)
		}
	}
}

type DataResult int

const (
	ValidIP DataResult = iota
	InvalidIP
	NS
)

func verifyIPs(trip Triplet, ipCertMap IPCertMap) {
	for _, pair := range trip {
		for _, single := range pair {
			for _, eventPtr := range single {
				for dom, ips := range eventPtr.Data {
					var domResult DataResult
					for _, ip := range ips {
						if cert, ok := ipCertMap[ip]; ok {
							err := cert.VerifyHostname(dom)
							if err != nil {
								domResult = InvalidIP
							} else {
								if dom == "facebook.com" || dom == "twitter.com" {
									continue
								}
								domResult = ValidIP
								infoLogger.Printf("Got valid IP for %s: %s\n", dom, ip)
								break
							}
						} else {
							if net.ParseIP(ip) != nil {
								domResult = InvalidIP
							} else {
								domResult = NS
							}
						}
					}
					switch domResult {
					case ValidIP:
						eventPtr.ValidIP++
					case InvalidIP:
						eventPtr.InvalidIP++
					case NS:
						eventPtr.ValidNS++
					}
				}

			}
		}
	}

}

type EventTable struct {
	ValidIP   int
	InvalidIP int
	Timeout   int
	NoAns     int
	NS        int
}

func (et EventTable) Total() int {
	var total int

	total += et.ValidIP
	total += et.InvalidIP
	total += et.Timeout
	total += et.NoAns
	total += et.NS

	return total
}

func getTable(trip Triplet) (EventTable, EventTable) {
	var (
		v4EventTable EventTable
		v6EventTable EventTable
	)

	for _, pair := range trip {
		for _, single := range pair {
			for vType, eventPtr := range single {
				switch vType {
				case "v4":
					v4EventTable.ValidIP += eventPtr.ValidIP
					v4EventTable.InvalidIP += eventPtr.InvalidIP
					v4EventTable.Timeout += eventPtr.Timeout
					v4EventTable.NoAns += eventPtr.NoAns
					v4EventTable.NS += eventPtr.ValidNS
					// this should be zero atm, but for future use maybe
					v4EventTable.NS += eventPtr.InvalidNS
				case "v6":
					v6EventTable.ValidIP += eventPtr.ValidIP
					v6EventTable.InvalidIP += eventPtr.InvalidIP
					v6EventTable.Timeout += eventPtr.Timeout
					v6EventTable.NoAns += eventPtr.NoAns
					v6EventTable.NS += eventPtr.ValidNS
					// this should be zero atm, but for future use maybe
					v6EventTable.NS += eventPtr.InvalidNS
				}

			}
		}
	}

	return v4EventTable, v6EventTable
}

func printTable(v4Table, v6Table EventTable) {
	fmt.Printf("\t\t| v4\t| v6\t| Total\n")
	fmt.Printf("ValidIP\t\t| %d\t| %d\t| %d\n", v4Table.ValidIP, v6Table.ValidIP, v4Table.ValidIP+v6Table.ValidIP)
	fmt.Printf("InvalidIP\t| %d\t| %d\t| %d\n", v4Table.InvalidIP, v6Table.InvalidIP, v4Table.InvalidIP+v6Table.InvalidIP)
	fmt.Printf("Timeout\t\t| %d\t| %d\t| %d\n", v4Table.Timeout, v6Table.Timeout, v4Table.Timeout+v6Table.Timeout)
	fmt.Printf("NoAns\t\t| %d\t| %d\t| %d\n", v4Table.NoAns, v6Table.NoAns, v4Table.NoAns+v6Table.NoAns)
	fmt.Printf("NS\t\t| %d\t| %d\t| %d\n", v4Table.NS, v6Table.NS, v4Table.NS+v6Table.NS)
	fmt.Printf("Total\t\t| %d\t| %d\t| %d\n", v4Table.Total(), v6Table.Total(), v4Table.Total()+v6Table.Total())
}

func printPTable(v4Table, v6Table EventTable) {
	v4Total := v4Table.Total()
	v6Total := v6Table.Total()
	denom := v4Total + v6Total
	fmt.Printf("\t\t| v4\t\t| v6\t\t| Total\n")
	validTotal := float64(v4Table.ValidIP + v6Table.ValidIP)
	v4ValidExpected := float64(v4Total) * (validTotal / float64(denom))
	v4ValidNum := float64(v4Table.ValidIP) - v4ValidExpected
	v4ValidNum = v4ValidNum * v4ValidNum
	v4Valid := v4ValidNum / v4ValidExpected
	v4Valid = v4Valid / 100.0
	v6ValidExpected := float64(v6Total) * (validTotal / float64(denom))
	v6ValidNum := float64(v6Table.ValidIP) - v6ValidExpected
	v6ValidNum = v6ValidNum * v6ValidNum
	v6Valid := v6ValidNum / v6ValidExpected
	v6Valid = v6Valid / 100.0
	if math.IsNaN(v4Valid) {
		v4Valid = 0.0
	}
	if math.IsNaN(v6Valid) {
		v6Valid = 0.0
	}
	fmt.Printf("ValidIP\t\t| %f\t| %f\t| %f\n", v4Valid, v6Valid, v4Valid+v6Valid)
	invalidTotal := float64(v4Table.InvalidIP + v6Table.InvalidIP)
	v4InvalidExpected := float64(v4Total) * (invalidTotal / float64(denom))
	v4InvalidNum := float64(v4Table.InvalidIP) - v4InvalidExpected
	v4InvalidNum = v4InvalidNum * v4InvalidNum
	v4Invalid := v4InvalidNum / v4InvalidExpected
	v4Invalid = v4Invalid / 100.0
	v6InvalidExpected := float64(v6Total) * (invalidTotal / float64(denom))
	v6InvalidNum := float64(v6Table.InvalidIP) - v6InvalidExpected
	v6InvalidNum = v6InvalidNum * v6InvalidNum
	v6Invalid := v6InvalidNum / v6InvalidExpected
	v6Invalid = v6Invalid / 100.0
	fmt.Printf("InvalidIP\t| %f\t| %f\t| %f\n", v4Invalid, v6Invalid, v4Invalid+v6Invalid)
	timeoutTotal := float64(v4Table.Timeout + v6Table.Timeout)
	v4TimeoutExpected := float64(v4Total) * (timeoutTotal / float64(denom))
	v4TimeoutNum := float64(v4Table.Timeout) - v4TimeoutExpected
	v4TimeoutNum = v4TimeoutNum * v4TimeoutNum
	v4Timeout := v4TimeoutNum / v4TimeoutExpected
	v4Timeout = v4Timeout / 100.0
	v6TimeoutExpected := float64(v6Total) * (timeoutTotal / float64(denom))
	v6TimeoutNum := float64(v6Table.Timeout) - v6TimeoutExpected
	v6TimeoutNum = v6TimeoutNum * v6TimeoutNum
	v6Timeout := v6TimeoutNum / v6TimeoutExpected
	v6Timeout = v6Timeout / 100.0
	fmt.Printf("Timeout\t\t| %f\t| %f\t| %f\n", v4Timeout, v6Timeout, v4Timeout+v6Timeout)
	noAnsTotal := float64(v4Table.NoAns + v6Table.NoAns)
	v4NoAnsExpected := float64(v4Total) * (noAnsTotal / float64(denom))
	v4NoAnsNum := float64(v4Table.NoAns) - v4NoAnsExpected
	v4NoAnsNum = v4NoAnsNum * v4NoAnsNum
	v4NoAns := v4NoAnsNum / v4NoAnsExpected
	v4NoAns = v4NoAns / 100.0
	v6NoAnsExpected := float64(v6Total) * (noAnsTotal / float64(denom))
	v6NoAnsNum := float64(v6Table.NoAns) - v6NoAnsExpected
	v6NoAnsNum = v6NoAnsNum * v6NoAnsNum
	v6NoAns := v6NoAnsNum / v6NoAnsExpected
	v6NoAns = v6NoAns / 100.0
	fmt.Printf("NoAns\t\t| %f\t| %f\t| %f\n", v4NoAns, v6NoAns, v4NoAns+v6NoAns)
	nsTotal := float64(v4Table.NS + v6Table.NS)
	v4NSExpected := float64(v4Total) * (nsTotal / float64(denom))
	v4NSNum := float64(v4Table.NS) - v4NSExpected
	v4NSNum = v4NSNum * v4NSNum
	v4NS := v4NSNum / v4NSExpected
	v4NS = v4NS / 100.0
	v6NSExpected := float64(v6Total) * (nsTotal / float64(denom))
	v6NSNum := float64(v6Table.NS) - v6NSExpected
	v6NSNum = v6NSNum * v6NSNum
	v6NS := v6NSNum / v6NSExpected
	v6NS = v6NS / 100.0
	fmt.Printf("NS\t\t| %f\t| %f\t| %f\n", v4NS, v6NS, v4NS+v6NS)
	v4TableTotal := v4Valid + v4Invalid + v4Timeout + v4NoAns + v4NS
	v6TableTotal := v6Valid + v6Invalid + v6Timeout + v6NoAns + v6NS
	fmt.Printf("Total:\t\t| %f\t| %f\t| %f\n", v4TableTotal, v6TableTotal, v4TableTotal+v6TableTotal)
}

func main() {
	resultsPath := flag.String("r", "", "Path to results file")
	uncensoredDomains := flag.String("u", "", "comma separated list of domains that are uncesnsored")
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
	doms := strings.Split(*uncensoredDomains, ",")
	if len(doms) == 0 {
		infoLogger.Printf(
			"No commas in uncensored domains list, using %s as one domain\n",
			*uncensoredDomains,
		)
		doms = []string{*uncensoredDomains}
	}
	infoLogger.Printf("Uncensored Domains: %v\n", doms)
	var wg sync.WaitGroup
	dataInChan := make(chan string)
	ipCertChan := make(chan *IPandCert)
	ipCertMapChan := make(chan IPCertMap)
	go checkData(dataInChan, ipCertChan, &wg)
	go collectIPResults(ipCertChan, ipCertMapChan)

	fullProbeResults := getStruct(*resultsPath)
	restTriplet, restOpenTriplet, uncensoredTriplet, uncensoredOpenTriplet :=
		resultsToTriplets(fullProbeResults, doms, dataInChan)
	close(dataInChan)
	infoLogger.Printf("Waiting to TLS lookups to finish")
	wg.Wait()
	close(ipCertChan)
	ipCertMap := <-ipCertMapChan
	infoLogger.Printf("got results for %d ips\n", len(ipCertMap))
	infoLogger.Printf("Verifying ips/domains\n")
	verifyIPs(restTriplet, ipCertMap)
	verifyIPs(restOpenTriplet, ipCertMap)
	verifyIPs(uncensoredTriplet, ipCertMap)
	verifyIPs(uncensoredOpenTriplet, ipCertMap)
	infoLogger.Printf("Generating event x (v4/v6) tables\n")
	v4RestTable, v6RestTable := getTable(restTriplet)
	v4RestOpenTable, v6RestOpenTable := getTable(restOpenTriplet)
	v4UncensoredTable, v6UncensoredTable := getTable(uncensoredTriplet)
	v4UncensoredOpenTable, v6UncensoredOpenTable := getTable(uncensoredOpenTriplet)
	infoLogger.Printf("'Censored' Domains, Domain Resolver Table\n")
	printTable(v4RestTable, v6RestTable)
	infoLogger.Printf("'Censored' Domains, Open Resolver Table\n")
	printTable(v4RestOpenTable, v6RestOpenTable)
	infoLogger.Printf("%v, Domain Resolver Table\n", doms)
	printTable(v4UncensoredTable, v6UncensoredTable)
	infoLogger.Printf("%v, Open Resolver Table\n", doms)
	printTable(v4UncensoredOpenTable, v6UncensoredOpenTable)
	// infoLogger.Printf("'Censored' Domains p-Table\n")
	// printPTable(v4RestTable, v6RestTable)
	// infoLogger.Printf("%v p-Table\n", )
	// printPTable(v4UncensoredTable, v6UncensoredTable)
}
