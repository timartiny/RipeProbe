package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"github.com/oschwald/geoip2-golang"
	experiment "github.com/timartiny/RipeProbe/RipeExperiment"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

// Gets data from path, assumes its JSON data of []experiment.LookupResult form.
func getData(path string) []experiment.LookupResult {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Can't open file (%s) with JSON data, %v\n", path, err)
	}
	defer file.Close()

	jsonBytes, err := ioutil.ReadAll(file)
	if err != nil {
		errorLogger.Fatalf("Can't read data from file (%s), %v\n", path, err)
	}
	var ret []experiment.LookupResult
	err = json.Unmarshal(jsonBytes, &ret)
	if err != nil {
		errorLogger.Fatalf("Can't unmarhsal JSON bytes, %v\n", err)
	}

	return ret
}

// Gets the important "resolver" data by finding each ip associated with each domain
func getDomainsAndIPs(data []experiment.LookupResult) map[string]string {
	ipsToDomain := make(map[string]string)
	for _, lr := range data {
		for _, lv4 := range lr.LocalV4 {
			ipsToDomain[lv4] = lr.Domain
		}
		for _, lv6 := range lr.LocalV6 {
			ipsToDomain[lv6] = lr.Domain
		}

		for _, rr := range lr.RipeResults {
			for _, rv4 := range rr.V4 {
				ipsToDomain[rv4] = lr.Domain
			}
			for _, rv6 := range rr.V6 {
				ipsToDomain[rv6] = lr.Domain
			}
		}
	}

	return ipsToDomain
}

// remove non cc country IPs by using a geoip2/geolite2 DB to look up IPs
func removeNonCountryIPs(cc string, ipMap map[string]string, dbPath string) {
	if len(cc) == 0 {
		infoLogger.Printf("No country code provided (-c) so keeping all ips\n")
		return
	}
	db, err := geoip2.Open(dbPath)
	if err != nil {
		errorLogger.Fatalf("Failed to open database: %s, %v\n", dbPath, err)
	}
	defer db.Close()

	for k := range ipMap {
		ip := net.ParseIP(k)
		record, err := db.Country(ip)
		if err != nil {
			errorLogger.Fatalf("error finding country code for %s, %v\n", k, err)
		}

		if record.Country.IsoCode != cc {
			delete(ipMap, k)
		}
	}

}

func writeData(ipMap map[string]string, outPath string) {
	file, err := os.Create(outPath)
	if err != nil {
		errorLogger.Fatalf("failed to create file: %s\nerr: %v\ndata: %v\n", outPath, err, ipMap)
	}
	defer file.Close()

	for ip, dom := range ipMap {
		file.WriteString(fmt.Sprintf("%s %s\n", ip, dom))
	}
}

func addResolvers(orPath string, ipMap map[string]string, cc string, num int) {
	orFile, err := os.Open(orPath)
	if err != nil {
		errorLogger.Printf("error opening file: %s, %v\n", orPath, err)
		return
	}
	defer orFile.Close()

	resolverDom := fmt.Sprintf("%s_Resolver", cc)
	orScanner := bufio.NewScanner(orFile)
	for orScanner.Scan() && num > 0 {
		split := strings.Split(orScanner.Text(), "  ")
		if split[2] == cc {
			ipMap[split[0]] = resolverDom
			ipMap[split[1]] = resolverDom
		}
	}
}

func main() {
	const NUMRESOLVERS = 5
	jsonPath := flag.String("lookup", "", "Path to JSON file that has measurement data")
	countryCode := flag.String("c", "", "Country code to check IPs against")
	openResolverPath := flag.String("resolvers", "", "Path to file containing open resolvers that are assumed to be correct, with country code")
	outPath := flag.String("out", "", "Path to write resolver list to")
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
	data := getData(*jsonPath)
	ipsToDomain := getDomainsAndIPs(data)
	removeNonCountryIPs(*countryCode, ipsToDomain, "data/geolite-country.mmdb")
	if len(*openResolverPath) > 0 {
		infoLogger.Printf("Now will add open resolvers, should be very quick\n")
		addResolvers(*openResolverPath, ipsToDomain, *countryCode, NUMRESOLVERS)
	}
	writeData(ipsToDomain, *outPath)
}
