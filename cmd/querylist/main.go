package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
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

// This function will return a popular domain that isn't blocked, or an error
func getUnblockedDomain(popDoms []string, blockedDoms map[string]interface{}, num int) ([]string, error) {
	var ret []string
	for _, dom := range popDoms {
		if _, ok := blockedDoms[dom]; !ok {
			ret = append(ret, dom)
		}

		if len(ret) == num {
			return ret, nil
		}
	}

	return []string{""}, errors.New("failed to find a popular domain that wasn't blocked")
}

// Will get requested number of popDoms from blockedDoms (or as many as possible)
func getNBlockedPopularDomains(popDoms []string, blockedDoms map[string]interface{}, num int) []string {
	var ret []string
	for _, dom := range popDoms {
		if _, ok := blockedDoms[dom]; ok {
			ret = append(ret, dom)
		}

		if len(ret) == num {
			break
		}
	}

	return ret
}

func writeToFile(list []string, path string) {
	file, err := os.Create(path)
	if err != nil {
		errorLogger.Printf("Can't open file, printing list, %v", err)
		fmt.Printf("%v\n", list)
		os.Exit(1)
	}
	defer file.Close()

	for _, dom := range list {
		_, err = file.WriteString(dom + "\n")
		if err != nil {
			errorLogger.Printf("Can't write to file, printing list, %v\n", err)
			fmt.Printf("%v\n", list)
			os.Exit(2)
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
	const AT_A_TIME = 100
	blockedPath := flag.String(
		"b",
		"",
		"Path to file containing special domains, possibly blocked",
	)
	blockedNum := flag.Int(
		"n",
		4,
		"Number of blocked domains to print",
	)
	unblockedNum := flag.Int(
		"u",
		1,
		"Number of unblocked domains to print",
	)

	flag.Parse()
	infoLogger.Printf("Getting 'blocked' domains from %s\n", *blockedPath)
	blockedDoms := getBlocked(*blockedPath)
	var final []string
	skip := 0
	neededUnblocked := *unblockedNum
	neededBlocked := *blockedNum

	for neededUnblocked > 0 || neededBlocked > 0 {
		infoLogger.Printf(
			"Grabbing %d popular domains, numbers %d-%d from %s\n",
			AT_A_TIME,
			skip+1,
			skip+AT_A_TIME,
			*popularityPath,
		)
		nPopDoms := getNPopular(*popularityPath, AT_A_TIME, skip)
		if neededUnblocked > 0 {
			infoLogger.Printf("Looking for popular unblocked domain(s)\n")
			doms, err := getUnblockedDomain(nPopDoms, blockedDoms, neededUnblocked)
			if err == nil {
				infoLogger.Printf("Found popular unblocked domain(s): %s\n", doms)
				final = append(final, doms...)
				neededUnblocked -= len(doms)
			}
		}

		if neededBlocked > 0 {
			infoLogger.Printf(
				"Looking for %d popular blocked domains\n",
				neededBlocked,
			)
			doms := getNBlockedPopularDomains(nPopDoms, blockedDoms, neededBlocked)
			infoLogger.Printf(
				"Found %d popular blocked domains: %v",
				len(doms),
				doms,
			)
			final = append(final, doms...)
			neededBlocked -= len(doms)
		}

		skip += AT_A_TIME
	}

	if len(*countryCode) == 0 {
		errorLogger.Fatalf("Need to provide country code (-c) to save to file\n")
	}
	outPath := fmt.Sprintf("%s/%s_bad_domains.dat", dataPrefix, *countryCode)
	infoLogger.Printf("Writing final list: %v to file %s\n", final, outPath)

	writeToFile(final, outPath)
}
