package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strconv"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

func readIds(file string) []int {
	if len(file) <= 0 {
		errorLogger.Fatalf(
			"Need to provide a file to read for measurement IDs, " +
				"use --idFile flag",
		)
	}
	f, err := os.Open(file)
	if err != nil {
		errorLogger.Fatalf("Error opening ids file: %v\n", err)
	}
	defer f.Close()

	var ids []int

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		idInt, err := strconv.Atoi(scanner.Text())
		if err != nil {
			errorLogger.Fatalf("error converting Ids to ints, %v\n", err)
		}
		ids = append(ids, idInt)
	}

	return ids
}

func main() {
	idsFile := flag.String(
		"idFile",
		"",
		"The file containing measurement Ids to look up and parse",
	)
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

	ids := readIds(*idsFile)
	infoLogger.Printf("Will look up results for measurements: %v\n", ids)
}
