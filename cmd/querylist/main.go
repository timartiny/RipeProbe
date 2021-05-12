package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var infoLogger *log.Logger
var errorLogger *log.Logger

// Assumes file has form: rank,domain
// such as Tranco list
func getNPopular(path string, needed, skip int) []string {
	var ret []string
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf(
			"Error opening popularity file, %s, exiting: %v\n",
			path,
			err,
		)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for i := 0; i < skip; i++ {
		if scanner.Scan() {
			scanner.Text()
		} else {
			errorLogger.Fatalf(
				"Not enough lines to skip %d lines, exiting\n",
				skip,
			)
		}
	}

	for i := 0; i < needed; i++ {
		if scanner.Scan() {
			text := scanner.Text()
			split := strings.Split(text, ",")
			ret = append(ret, split[1])
		} else {
			errorLogger.Printf(
				"Ran out of lines before getting %d lines, returning with %d "+
					"lines",
				needed,
				len(ret),
			)
			return ret
		}
	}

	return ret
}

// removes protocol and www. subdomains and ending slash
func strip(url string) string {
	protoIndex := strings.Index(url, "://")
	var hostAndPath string
	if protoIndex != -1 {
		hostAndPath = url[protoIndex+3:]
	}
	wwwIndex := strings.Index(hostAndPath, "www.")
	if wwwIndex != -1 {
		hostAndPath = hostAndPath[wwwIndex+4:]
	}
	hostAndPathLen := len(hostAndPath)
	if hostAndPathLen > 0 && hostAndPath[hostAndPathLen-1] == '/' {
		hostAndPath = hostAndPath[:hostAndPathLen-1]
	}

	return hostAndPath
}

// assumes file has form:
// url,category_code,category_description,date_added,source,notes
// url has protocol (http[s]), might have www. and an ending slash
// this file will remove them all.
func getBlocked(path string) map[string]interface{} {
	ret := make(map[string]interface{})
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Can't open file, %s, %v\n", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		// clear first line, citizen lab adds a header line
		scanner.Text()
	} else {
		errorLogger.Fatalf("File didn't have any lines\n")
	}

	for scanner.Scan() {
		line := scanner.Text()
		url := strings.Split(line, ",")[0]
		strippedUrl := strip(url)
		ret[strippedUrl] = nil
	}

	return ret
}

func hasV4AndV6(dom string) (bool, bool) {
	var (
		hasV4 bool
		hasV6 bool
	)
	r := &net.Resolver{
		PreferGo: false,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, net.JoinHostPort("8.8.8.8", "53"))
		},
	}
	ip4s, err := r.LookupIP(context.Background(), "ip4", dom)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") {
			errorLogger.Printf("r.LookupIP(.,ip4,.) err: %v", err)
		}
	}
	if len(ip4s) > 0 {
		hasV4 = true
	}
	ip6s, err := r.LookupIP(context.Background(), "ip6", dom)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") {
			errorLogger.Printf("r.LookupIP(.,ip6,.) err: %v", err)
		}
	}

	if len(ip6s) > 0 {
		hasV6 = true
	}

	return hasV4, hasV6
}

func writeToFile(im InterestingMap, path string) {
	file, err := os.Create(path)
	if err != nil {
		errorLogger.Printf("Can't open file %s: %v", path, err)
		os.Exit(1)
	}
	defer file.Close()
	writeSlice := make([]*DomainResults, len(im))

	idx := 0
	for _, dr := range im {
		writeSlice[idx] = dr
		idx++
	}

	bs, err := json.Marshal(&writeSlice)
	if err != nil {
		errorLogger.Printf("json.Marshal error: %v\n", err)
		os.Exit(2)
	}
	file.Write(bs)
}

type DomainResults struct {
	Domain  string `json:"domain,omitempty"`
	HasV4   bool   `json:"has_v4"`
	HasV6   bool   `json:"has_v6"`
	HasTLS  bool   `json:"has_tls"`
	Blocked bool   `json:"blocked"`
}

func hasTLS(domain string) bool {
	config := tls.Config{ServerName: domain}
	timeout := time.Duration(90) * time.Second
	dialConn, err := net.DialTimeout(
		"tcp", net.JoinHostPort(domain, "443"), timeout,
	)
	if err != nil {
		// errorLogger.Printf("net.DialTimeout err: %v\n", err)
		return false
	}
	tlsConn := tls.Client(dialConn, &config)
	defer tlsConn.Close()
	dialConn.SetReadDeadline(time.Now().Add(timeout))
	dialConn.SetWriteDeadline(time.Now().Add(timeout))

	tlsConn.Handshake()
	err = tlsConn.VerifyHostname(domain)
	if err != nil {
		// errorLogger.Printf("tlsConn.VerifyHostname err: %v\n", err)
		return false
	}

	return tlsConn.ConnectionState().PeerCertificates[0].NotAfter.After(time.Now())
}

func checkDom(domResultsChan chan<- *DomainResults, dom string) {
	dr := new(DomainResults)
	dr.Domain = dom
	h4, h6 := hasV4AndV6(dom)
	dr.HasV4 = h4
	dr.HasV6 = h6
	dr.HasTLS = hasTLS(dom)

	domResultsChan <- dr
}

func domChecker(domInChan <-chan string, domResultsChan chan<- *DomainResults) {
	for dom := range domInChan {
		dr := new(DomainResults)
		dr.Domain = dom
		h4, h6 := hasV4AndV6(dom)
		dr.HasV4 = h4
		dr.HasV6 = h6
		dr.HasTLS = hasTLS(dom)

		domResultsChan <- dr
	}
}

type InterestingMap map[string]*DomainResults

func domResults(
	domResultsChan <-chan *DomainResults,
	interestingChan chan<- InterestingMap,
	counterChan chan<- string,
	wg *sync.WaitGroup,
) {
	interestingMap := make(InterestingMap)
	counter := 0
	for dr := range domResultsChan {
		counter++
		// if dr.HasTLS && dr.HasV4 && dr.HasV6 {
		// }
		interestingMap[dr.Domain] = dr
		if counter%100 == 0 {
			infoLogger.Printf("got results from %d domains\n", counter)
		}
		// counterChan <- dr.Domain
		wg.Done()
	}
	interestingChan <- interestingMap
}

func updateInterestingMap(im InterestingMap, blockedDoms map[string]interface{}) {
	for dom, dr := range im {
		_, ok := blockedDoms[dom]
		dr.Blocked = ok
	}
}

func counter(cc <-chan string) {
	cm := make(map[string]interface{})
	for dom := range cc {
		if _, ok := cm[dom]; ok {
			delete(cm, dom)
		} else {
			cm[dom] = nil
		}

		if len(cm) <= 10 {
			infoLogger.Printf("remaining doms: %v\n", cm)
		}
	}
}

func main() {
	dataPrefix := "../../data"
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

	countryCode := flag.String("c", "", "Country code, used to save file")
	// Currently using Tranco lists which take a significant amount of time to
	// fully load, so we will load 100 at a time, and only more if needed
	popularityPath := flag.String(
		"p",
		"",
		"Path to file listing domains by popularity",
	)
	cap := flag.Int(
		"cap",
		-1,
		"Maximum number of domains to read from popular file",
	)
	blockedPath := flag.String(
		"b",
		"",
		"Path to file containing special domains, possibly blocked",
	)
	numWorkers := flag.Int(
		"w",
		5,
		"Number of separate goroutines to do look-ups",
	)
	// blockedNum := flag.Int(
	// 	"n",
	// 	-1,
	// 	"Number of blocked domains to print, (negative number means grab all)",
	// )
	// unblockedNum := flag.Int(
	// 	"u",
	// 	-1,
	// 	"Number of unblocked domains to print, (negative number means grab all)",
	// )

	domResultsChan := make(chan *DomainResults)
	interestingChan := make(chan InterestingMap)
	domInChan := make(chan string)
	counterChan := make(chan string)
	var wg sync.WaitGroup

	flag.Parse()
	file, err := os.Open(*popularityPath)
	if err != nil {
		errorLogger.Fatalf(
			"Error opening popularity file, %s, exiting: %v\n",
			*popularityPath,
			err,
		)
	}
	defer file.Close()
	go domResults(domResultsChan, interestingChan, counterChan, &wg)
	// go counter(counterChan)
	scanner := bufio.NewScanner(file)
	for i := 0; i < *numWorkers; i++ {
		go domChecker(domInChan, domResultsChan)
	}

	counter := 0
	for scanner.Scan() {
		text := scanner.Text()
		split := strings.Split(text, ",")
		wg.Add(1)
		counter++
		// counterChan <- split[1]
		domInChan <- split[1]
		// go checkDom(domResultsChan, split[1])
		if *cap > 0 && counter >= *cap {
			break
		}
		if counter%100 == 0 {
			time.Sleep(time.Duration(5) * time.Second)
		}
	}
	infoLogger.Printf(
		"read in all %d popular domains. Waiting for interestingness\n", counter,
	)
	wg.Wait()
	close(domResultsChan)
	close(domInChan)
	close(counterChan)
	interestingMap := <-interestingChan
	// for dom := range interestingMap {
	// 	infoLogger.Printf("Interesting domain: %s\n", dom)
	// }
	infoLogger.Printf("Getting 'blocked' domains from %s\n", *blockedPath)
	blockedDoms := getBlocked(*blockedPath)
	updateInterestingMap(interestingMap, blockedDoms)

	if len(*countryCode) == 0 {
		errorLogger.Fatalf("Need to provide country code (-c) to save to file\n")
	}
	outPath := fmt.Sprintf("%s/%s_interesting_domains.dat", dataPrefix, *countryCode)

	infoLogger.Printf("Writing interesting map to %s\n", outPath)
	writeToFile(interestingMap, outPath)
}
