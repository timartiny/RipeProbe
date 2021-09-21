package main

import (
	"bufio"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	flags "github.com/zmap/zflags"
	"github.com/zmap/zgrab2"
)

var infoLogger *log.Logger
var errorLogger *log.Logger

type ZDNSResult struct {
	AlteredName string        `json:"altered_name,omitempty" groups:"short,normal,long,trace"`
	Name        string        `json:"name,omitempty" groups:"short,normal,long,trace"`
	Nameserver  string        `json:"nameserver,omitempty" groups:"normal,long,trace"`
	Class       string        `json:"class,omitempty" groups:"long,trace"`
	AlexaRank   int           `json:"alexa_rank,omitempty" groups:"short,normal,long,trace"`
	Metadata    string        `json:"metadata,omitempty" groups:"short,normal,long,trace"`
	Status      string        `json:"status,omitempty" groups:"short,normal,long,trace"`
	Error       string        `json:"error,omitempty" groups:"short,normal,long,trace"`
	Timestamp   string        `json:"timestamp,omitempty" groups:"short,normal,long,trace"`
	Data        interface{}   `json:"data,omitempty" groups:"short,normal,long,trace"`
	Trace       []interface{} `json:"trace,omitempty" groups:"trace"`
}

type ZDNSAnswer struct {
	Ttl     uint32 `json:"ttl" groups:"ttl,normal,long,trace"`
	Type    string `json:"type,omitempty" groups:"short,normal,long,trace"`
	RrType  uint16 `json:"-"`
	Class   string `json:"class,omitempty" groups:"short,normal,long,trace"`
	RrClass uint16 `json:"-"`
	Name    string `json:"name,omitempty" groups:"short,normal,long,trace"`
	Answer  string `json:"answer,omitempty" groups:"short,normal,long,trace"`
}

type IPSupportsTLS map[string]bool

type TLSResults struct {
	Domain    string
	Addresses IPSupportsTLS
}

type TLSResultsMap map[string]*TLSResults

type DomainResults struct {
	Domain                string   `json:"domain,omitempty"`
	Rank                  int      `json:"tranco_rank,omitempty"`
	HasV4                 bool     `json:"has_v4"`
	HasV6                 bool     `json:"has_v6"`
	HasV4TLS              bool     `json:"has_v4_tls"`
	HasV6TLS              bool     `json:"has_v6_tls"`
	CitizenLabGlobalList  bool     `json:"citizen_lab_global_list"`
	CitizenLabCountryList []string `json:"citizen_lab_country_list"`
}

type DomainResultsMap map[string]*DomainResults

type QuerylistFlags struct {
	V4DNS               string `long:"v4_dns" description:"Path to the ZDNS results for v4 lookups" required:"true" json:"v4_dns"`
	V6DNS               string `long:"v6_dns" description:"Path to the ZDNS results for v6 lookups" required:"true" json:"v6_dns"`
	V4TLS               string `long:"v4_tls" description:"Path to the ZGrab results for v4 TLS banner grabs" required:"true" json:"v4_tls"`
	V6TLS               string `long:"v6_tls" description:"Path to the ZGrab results for v6 TLS banner grabs" required:"true" json:"v6_tls"`
	CitizenLabDirectory string `long:"citizen_lab_directory" description:"Path to the directory containing the Citizen Lab lists" required:"true" json:"citizen_lab_directory"`
	Outfile             string `long:"out_file" description:"File to write all details to (in JSON)" required:"true" json:"out_file"`
}

// removes protocol and www. subdomains and ending slash
func stripUrl(url string) string {
	protoIndex := strings.Index(url, "://")
	var hostAndPath string
	if protoIndex != -1 {
		hostAndPath = url[protoIndex+3:]
	} else {
		hostAndPath = url
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
		strippedUrl := stripUrl(url)
		ret[strippedUrl] = nil
	}

	return ret
}

// writeToFile will write the results map to the provided file
// one line of JSON at a time.
func writeToFile(drm DomainResultsMap, path string) {
	infoLogger.Printf("Writing to %s\n", path)
	file, err := os.Create(path)
	if err != nil {
		errorLogger.Printf("Can't open file %s: %v", path, err)
		os.Exit(1)
	}
	defer file.Close()

	for _, dr := range drm {
		bs, err := json.Marshal(dr)
		if err != nil {
			errorLogger.Printf("json.Marshal error: %v\n", err)
			os.Exit(2)
		}
		file.WriteString(string(bs) + "\n")
	}

}

// setupArgs grabs the commandline arguments and puts them in a usable struct
func setupArgs(args []string) QuerylistFlags {
	var ret QuerylistFlags
	posArgs, _, _, err := flags.ParseArgs(&ret, args)

	if err != nil {
		errorLogger.Printf("Error parsing args: %v\n", err)
		os.Exit(1)
	}
	if len(posArgs) > 0 {
		infoLogger.Printf("Extra arguments provided, but not used: %v\n", args)
	}

	return ret
}

// addDNSResults will take a path to the DNS results (in ZDNS form) and update
// drm.
func addDNSResults(drm DomainResultsMap, path string) {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Printf("os.Open err: %v\n", err)
		errorLogger.Fatalln("Please provide a valid file using the --v{4,6}_dns flag")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var zdnsResult ZDNSResult
		l := scanner.Text()
		json.Unmarshal([]byte(l), &zdnsResult)
		domainName := zdnsResult.Name
		// infoLogger.Printf("the domain name is: %s\n", domainName)
		if _, ok := drm[domainName]; !ok {
			// this domain is not already in the mapping, add it now.
			tmp := new(DomainResults)
			tmp.Domain = domainName
			tmp.Rank = zdnsResult.AlexaRank
			drm[domainName] = tmp
		}
		interfaceAnswers, ok := zdnsResult.Data.(map[string]interface{})["answers"]
		if !ok {
			// infoLogger.Printf("This results has no answers, domain: %s\n", domainName)
			continue
		}
		zdnsAnswers := interfaceAnswers.([]interface{})
		for _, interfaceAnswer := range zdnsAnswers {
			tmpJSONString, _ := json.Marshal(interfaceAnswer)
			var answer ZDNSAnswer
			json.Unmarshal(tmpJSONString, &answer)
			if answer.Type == "A" {
				ip := net.ParseIP(answer.Answer)
				if ip != nil && ip.To4() != nil {
					drm[domainName].HasV4 = true
					break
				}
			} else if answer.Type == "AAAA" {
				ip := net.ParseIP(answer.Answer)
				if ip != nil && ip.To4() == nil {
					drm[domainName].HasV6 = true
				}
			}
		}
	}
}

// addTLSResults will take the results of ZGrab2 tls banner grab and store
// collection of IPs and whether they have a valid TLS cert in trm
func addTLSResults(trm TLSResultsMap, path string) {
	file, err := os.Open(path)
	if err != nil {
		errorLogger.Printf("os.Open err: %v\n", err)
		errorLogger.Fatalln("Please provide a valid file using the --v{4,6}_tls flag")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var zgrabResult zgrab2.Grab
		l := scanner.Text()
		json.Unmarshal([]byte(l), &zgrabResult)
		domainName := zgrabResult.Domain
		if _, ok := trm[domainName]; !ok {
			// this domain is not already in the mapping, add it now.
			tmpTLSResults := new(TLSResults)
			tmpTLSResults.Domain = domainName

			tmpTLSResults.Addresses = IPSupportsTLS{zgrabResult.IP: false}
			trm[domainName] = tmpTLSResults
		} else {
			trm[domainName].Addresses[zgrabResult.IP] = false
		}

		tlsScanResults, ok := zgrabResult.Data["tls"]
		if !ok {
			// infoLogger.Printf("This results has no tls section, domain: %s\n", domainName)
			continue
		}
		if tlsScanResults.Status != "success" {
			// infoLogger.Printf("This results is a non-successful tls result: %s, %s\n", domainName, tlsScanResults.Status)
			continue
		}

		timestampString := tlsScanResults.Timestamp
		timestamp, err := time.Parse(time.RFC3339, timestampString)
		if err != nil {
			errorLogger.Printf("Error parsing timestamp: %s\n", timestampString)
			continue
		}
		serverHandshakeInterface := tlsScanResults.Result.(map[string]interface{})["handshake_log"].(map[string]interface{})
		serverCertificatesInterface := serverHandshakeInterface["server_certificates"].(map[string]interface{})
		leafCertificateInterface := serverCertificatesInterface["certificate"].(map[string]interface{})
		rawLeafCertificate := leafCertificateInterface["raw"].(string)

		decoded, err := base64.StdEncoding.DecodeString(string(rawLeafCertificate))
		if err != nil {
			errorLogger.Printf("base64.Decode of cert err: %v\n", err)
			continue
		}
		x509Cert, err := x509.ParseCertificate(decoded)
		if err != nil {
			errorLogger.Printf("x509.ParseCertificate of cert err: %v\n", err)
			errorLogger.Printf("decoded certificate: %v\n", decoded)
			continue
		}
		err = x509Cert.VerifyHostname(domainName)
		if err != nil {
			continue
		}

		certPool := x509.NewCertPool()
		var chain []interface{}
		if serverCertificatesInterface["chain"] != nil {
			chain = serverCertificatesInterface["chain"].([]interface{})
		}

		for ind, mInterface := range chain {
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

		verifyOptions := x509.VerifyOptions{
			DNSName:       domainName,
			CurrentTime:   timestamp,
			Intermediates: certPool,
		}
		_, err = x509Cert.Verify(verifyOptions)
		if err == nil {
			trm[domainName].Addresses[zgrabResult.IP] = true
		}
	}
}

// parseTLSResults will go throw the TLSResultsMap to determine whether a domain
// supports TLS	on either v4 or v6. A domain supports TLS if all of the IPs do
// This will print out anomalies like only some IPs supporting TLS
func parseTLSResults(trm TLSResultsMap, drm DomainResultsMap) {
	for domain, tlsResults := range trm {
		var domainSupportsTLS *bool
		var addressType string
		for ipString, ipSupportsTLS := range tlsResults.Addresses {
			ip := net.ParseIP(ipString)
			if ip.To4() == nil {
				addressType = "AAAA"
			} else {
				addressType = "A"
			}
			if domainSupportsTLS == nil {
				// this is the first iteration through
				domainSupportsTLS = &ipSupportsTLS
			} else if ipSupportsTLS != *domainSupportsTLS {
				infoLogger.Printf(
					"Unusual Situation: not all IPs for domain %s have the"+
						" same value on supporting TLS: %#v\n",
					domain,
					tlsResults.Addresses,
				)
				*domainSupportsTLS = false
				break
			}
		}
		if _, ok := drm[domain]; !ok {
			// only for debugging when DNS results don't happen first
			continue
		}

		if addressType == "A" {
			drm[domain].HasV4TLS = *domainSupportsTLS
		} else if addressType == "AAAA" {
			drm[domain].HasV6TLS = *domainSupportsTLS
		}
	}
}

// Updates existing tech details with a given file's contents
func addCountryBlockage(drm DomainResultsMap, path, countryCode string) {
	blockedDomains := getBlocked(path)
	for dom, dr := range drm {
		if _, ok := blockedDomains[dom]; !ok {
			// this domain isn't on this country's blocked list, so nothing to add
			continue
		}

		if countryCode == "GLOBAL" {
			dr.CitizenLabGlobalList = true
		} else {
			dr.CitizenLabCountryList = append(dr.CitizenLabCountryList, countryCode)
		}
	}
}

// fillInCitizenLabData will read each of the lists from Citizen Lab and fill
// in for each domain
func fillInCitizenLabData(drm DomainResultsMap, path string) {
	files, err := os.ReadDir(path)
	if err != nil {
		errorLogger.Fatalf("Error checking Citizen Lab List directory: %v\n", err)
	}

	matcher, err := regexp.Compile("^[a-z]{2}.csv")
	if err != nil {
		errorLogger.Fatalf("Error with regex pattern: %v\n", err)
	}
	for _, file := range files {
		fileName := file.Name()
		if fileName == "global.csv" || matcher.Match([]byte(fileName)) {
			countryCode := strings.ToUpper(strings.Split(fileName, ".")[0])
			addCountryBlockage(drm, filepath.Join(path, fileName), countryCode)
		}
	}
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

	args := setupArgs(os.Args[1:])

	domainResultsMap := make(DomainResultsMap)
	infoLogger.Printf("Reading in v4 DNS query results from %s\n", args.V4DNS)
	addDNSResults(domainResultsMap, args.V4DNS)
	infoLogger.Printf("Google's results so far: %+v\n", domainResultsMap["google.com"])
	infoLogger.Printf("Netflix's results so far: %+v\n", domainResultsMap["netflix.com"])

	infoLogger.Printf("Reading in v6 DNS query results from %s\n", args.V6DNS)
	addDNSResults(domainResultsMap, args.V6DNS)
	infoLogger.Printf("Google's results so far: %+v\n", domainResultsMap["google.com"])
	infoLogger.Printf("Netflix's results so far: %+v\n", domainResultsMap["netflix.com"])

	tlsResultsMap := make(TLSResultsMap)
	infoLogger.Printf("Reading in v4 TLS banner grab results from %s\n", args.V4TLS)
	addTLSResults(tlsResultsMap, args.V4TLS)
	parseTLSResults(tlsResultsMap, domainResultsMap)
	infoLogger.Printf("Google's results so far: %+v\n", domainResultsMap["google.com"])
	infoLogger.Printf("Netflix's results so far: %+v\n", domainResultsMap["netflix.com"])

	tlsResultsMap = make(TLSResultsMap)
	infoLogger.Printf("Reading in v6 TLS banner grab results from %s\n", args.V6TLS)
	addTLSResults(tlsResultsMap, args.V6TLS)
	parseTLSResults(tlsResultsMap, domainResultsMap)
	infoLogger.Printf("Google's results so far: %+v\n", domainResultsMap["google.com"])
	infoLogger.Printf("Netflix's results so far: %+v\n", domainResultsMap["netflix.com"])

	infoLogger.Printf("Filling in Citizen Lab data from %s\n", args.CitizenLabDirectory)
	fillInCitizenLabData(domainResultsMap, args.CitizenLabDirectory)
	infoLogger.Printf("Google's final results: %+v\n", domainResultsMap["google.com"])
	infoLogger.Printf("Netflix's final results: %+v\n", domainResultsMap["netflix.com"])

	infoLogger.Printf("Writing all results to: %s\n", args.Outfile)
	writeToFile(domainResultsMap, args.Outfile)
}
