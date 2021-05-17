package main

import (
	"bufio"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
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
// this function will remove them all.
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

func writeToFile(im DomainResultsMap, path string) {
	infoLogger.Printf("Writing to %s\n", path)
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
	Domain                string `json:"domain,omitempty"`
	Rank                  int    `json:"tranco_rank,omitempty"`
	HasV4                 bool   `json:"has_v4"`
	HasV6                 bool   `json:"has_v6"`
	HasTLS                bool   `json:"has_tls"`
	CitizenLabGlobalList  bool   `json:"citizen_lab_global_list"`
	CitizenLabCountryList bool   `json:"citizen_lab_country_list"`
}

type DomainResultsMap map[string]*DomainResults

func updateDRM(drm DomainResultsMap, blockedDoms map[string]interface{}, list string) {
	for dom, dr := range drm {
		_, ok := blockedDoms[dom]
		switch list {
		case "citizenlab global":
			dr.CitizenLabGlobalList = ok
		case "citizenlab country":
			dr.CitizenLabCountryList = ok
		}
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

func mergeAddressResults(drm map[string]*DomainResults, path, addressType string) {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("os.Open err: %v\n", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	unCoveredCases := make(map[string]bool)

	for scanner.Scan() {
		var tmpMap map[string]interface{}
		var dr *DomainResults
		l := scanner.Text()
		json.Unmarshal([]byte(l), &tmpMap)
		domainName := tmpMap["name"].(string)
		tmpRank := int(tmpMap["alexa_rank"].(float64))

		// get existing or create the DomainResults
		if d, ok := drm[domainName]; ok {
			dr = d
			if dr.Rank != tmpRank {
				errorLogger.Fatalf(
					"Rank from %s for %s (%d) is different than previously "+
						"stored rank (%d)\n",
					path,
					domainName,
					tmpRank,
					dr.Rank,
				)
			}
		} else {
			dr = new(DomainResults)
			dr.Domain = domainName
			dr.Rank = tmpRank
		}

		// Now look at the data section
		data := tmpMap["data"].(map[string]interface{})

		// only look at the section called answers, not interested in authorities
		if answersInterface, ok := data["answers"]; ok {
			answersArr := answersInterface.([]interface{})
			for _, answerInterface := range answersArr {
				answerMap := answerInterface.(map[string]interface{})
				recordType := answerMap["type"].(string)

				// make sure to get matching record type based on query type
				if recordType == "A" && addressType == "v4" {
					ipString := answerMap["answer"].(string)
					ip := net.ParseIP(ipString)

					// consider domain to have v4 if the answer is actually
					// an ip address that can be converted to a v4 address
					if ip != nil && ip.To4() != nil {
						dr.HasV4 = true
					}
				} else if recordType == "AAAA" && addressType == "v6" {
					ipString := answerMap["answer"].(string)
					ip := net.ParseIP(ipString)

					// consider domain to have v4 if the answer is actually
					// an ip address that can be converted to a v4 address
					if ip != nil && ip.To4() == nil {
						dr.HasV6 = true
					}
				} else {
					uccString := fmt.Sprintf("%s,%s", recordType, addressType)
					if _, ok := unCoveredCases[uccString]; ok {
						continue
					}
					errorLogger.Printf(
						"Case not covered yet, got address type: %s and "+
							"record type: %s, for domain: %s\n",
						addressType,
						recordType,
						domainName,
					)
					unCoveredCases[uccString] = true
				}
			}
		}
		drm[domainName] = dr
	}
}

func mergeTLSResults(drm map[string]*DomainResults, path string) {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("os.Open err: %v\n", err)
	}
	defer file.Close()
	nonSuccessCase := map[string]bool{}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var tmpMap map[string]interface{}
		l := scanner.Text()
		json.Unmarshal([]byte(l), &tmpMap)
		domainName := tmpMap["domain"].(string)
		if _, ok := drm[domainName]; !ok {
			errorLogger.Printf(
				"domainName: %s, not already in drm, skipping for now, should "+
					"fix though\n",
				domainName,
			)
			continue
		}

		data := tmpMap["data"].(map[string]interface{})
		tlsData := data["tls"].(map[string]interface{})
		if tlsData["status"] != "success" {
			if _, ok := nonSuccessCase[tlsData["status"].(string)]; !ok {

				errorLogger.Printf(
					"tls status: %s for %s\n", tlsData["status"].(string), domainName,
				)
			}
			nonSuccessCase[tlsData["status"].(string)] = true
			continue
		}
		toCert := tlsData["result"].(map[string]interface{})
		toCert = toCert["handshake_log"].(map[string]interface{})
		toCert = toCert["server_certificates"].(map[string]interface{})
		cert := toCert["certificate"].(map[string]interface{})
		certRaw := cert["raw"].(string)
		decoded, err := base64.StdEncoding.DecodeString(certRaw)
		if err != nil {
			errorLogger.Printf("base64.Decode of cert err: %v\n", err)
			continue
		}
		x509Cert, err := x509.ParseCertificate(decoded)
		if err != nil {
			errorLogger.Printf("x509.ParseCertificate of cert err: %v\n", err)
			continue
		}
		err = x509Cert.VerifyHostname(domainName)
		if err != nil {
			errorLogger.Printf("cert doesn't match domain for %s\n", domainName)
		}
		certPool := x509.NewCertPool()
		var chain []interface{}
		if toCert["chain"] != nil {
			chain = toCert["chain"].([]interface{})
		}

		for ind, mInterface := range chain {
			// infoLogger.Printf("chain index: %d\n", ind)
			raw := mInterface.(map[string]interface{})["raw"]
			chainDecoded, err := base64.StdEncoding.DecodeString(raw.(string))
			if err != nil {
				errorLogger.Printf("base64.Decode of chain ind %d err: %v\n", ind, err)
				continue
			}

			chainCert, err := x509.ParseCertificate(chainDecoded)
			if err != nil {
				errorLogger.Printf("x509.ParseCertificate for chain ind %d err: %v\n", ind, err)
				continue
			}
			certPool.AddCert(chainCert)
		}
		_, err = x509Cert.Verify(x509.VerifyOptions{Intermediates: certPool})
		if err != nil {
			errorLogger.Printf("cert isn't valid for %s, %v\n", domainName, err)
			continue
		}
		drm[domainName].HasTLS = true
	}
}

func technicalRequirements(drm DomainResultsMap, v4Path, v6Path, tlsPath string) {
	infoLogger.Printf("Checking domains for v4 addresses\n")
	// read in results of v4 check
	mergeAddressResults(drm, v4Path, "v4")
	infoLogger.Printf("Sample Result: %#v\n", drm["google.com"])
	infoLogger.Printf("Checking domains for v6 addresses\n")
	// read in results of v6 check
	mergeAddressResults(drm, v6Path, "v6")
	infoLogger.Printf("Sample Result: %#v\n", drm["google.com"])
	// read in results of tls check
	mergeTLSResults(drm, tlsPath)
	infoLogger.Printf("Sample Result: %#v\n", drm["google.com"])
}

func checkBlockedLists(drm DomainResultsMap, citizenlabGlobal, citizenlabCountry string) {
	if len(citizenlabGlobal) == 0 {
		errorLogger.Printf("Citizen lab global list is empty, skipping.")
	} else {
		citizenlabMap := getBlocked(citizenlabGlobal)
		updateDRM(drm, citizenlabMap, "citizenlab global")
		infoLogger.Printf("Sample Result: %#v\n", drm["google.com"])
	}
	if len(citizenlabCountry) == 0 {
		errorLogger.Printf("Citizen lab country list is empty, skipping.")
	} else {
		citizenlabMap := getBlocked(citizenlabCountry)
		updateDRM(drm, citizenlabMap, "citizenlab country")
		infoLogger.Printf("Sample Result: %#v\n", drm["google.com"])
	}
}

func getTechRequirements(drm DomainResultsMap, path string) {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Fatalf("Can't open file, %s, %v\n", path, err)
	}
	defer file.Close()

	bs, err := ioutil.ReadAll(file)
	if err != nil {
		errorLogger.Fatalf("ioutil.Readall err on %s: %v\n", path, err)
	}
	var drSlice []*DomainResults
	err = json.Unmarshal(bs, &drSlice)
	if err != nil {
		errorLogger.Printf("json.Unmarshal error: %v\n", err)
	}

	for _, dr := range drSlice {
		drm[dr.Domain] = dr
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
	// // Currently using Tranco lists which take a significant amount of time to
	// // fully load, so we will load 100 at a time, and only more if needed
	v4Path := flag.String(
		"v4",
		"",
		"Path to file containing zdns results for v4",
	)
	v6Path := flag.String(
		"v6",
		"",
		"Path to file containing zdns results for v6",
	)
	tlsPath := flag.String(
		"tls",
		"",
		"Path to file containing zgrab2 results for tls",
	)
	techPath := flag.String(
		"tech",
		"",
		"Path to file containing technical requirements results",
	)
	// cap := flag.Int(
	// 	"cap",
	// 	-1,
	// 	"Maximum number of domains to read from popular file",
	// )
	clgPath := flag.String(
		"cit-lab-global",
		"",
		"Path to file containing special domains of interest globally",
	)
	clcPath := flag.String(
		"cit-lab-country",
		"",
		"Path to file containing special domains of interest to a specific country",
	)
	// numWorkers := flag.Int(
	// 	"w",
	// 	5,
	// 	"Number of separate goroutines to do look-ups",
	// )
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
	flag.Parse()

	domainResultsMap := make(DomainResultsMap)
	if len(*v4Path) == 0 || len(*v6Path) == 0 || len(*tlsPath) == 0 {
		infoLogger.Printf("One of the technical details paths was empty.\n")
		infoLogger.Printf("Trying the tech path\n")
		if len(*techPath) == 0 {
			errorLogger.Fatalln(
				"Additionlly the technical requirements path is empty. " +
					"One must be filled in to run\n",
			)
		}

		getTechRequirements(domainResultsMap, *techPath)
	} else {
		technicalRequirements(domainResultsMap, *v4Path, *v6Path, *tlsPath)
		writeToFile(
			domainResultsMap,
			fmt.Sprintf("%s/top-1m-tech-details.json", dataPrefix),
		)
	}

	checkBlockedLists(domainResultsMap, *clgPath, *clcPath)
	if len(*countryCode) != 0 {
		writeToFile(
			domainResultsMap,
			fmt.Sprintf(
				"%s/%s-top-1m-ripe-ready.json",
				dataPrefix,
				*countryCode,
			),
		)

	} else {
		errorLogger.Printf("No country code provided, saving to a generic file")
		writeToFile(
			domainResultsMap,
			fmt.Sprintf("%s/top-1m-ripe-ready.json", dataPrefix),
		)
	}
}
