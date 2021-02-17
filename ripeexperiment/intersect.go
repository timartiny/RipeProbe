package ripeexperiment

import (
	"encoding/csv"
	"os"
	"strings"

	"github.com/pkg/errors"
)

func getColumn(doubleList [][]string, columnID int) map[string]struct{} {
	column := make(map[string]struct{}, 0)
	for _, row := range doubleList {
		protoIndex := strings.Index(row[0], "://")
		var hostAndPath string
		if protoIndex != -1 {
			hostAndPath = row[0][protoIndex+3:]
		}
		wwwIndex := strings.Index(hostAndPath, "www.")
		if wwwIndex != -1 {
			hostAndPath = hostAndPath[wwwIndex+4:]
		}
		hostAndPathLen := len(hostAndPath)
		if hostAndPathLen > 0 && hostAndPath[hostAndPathLen-1] == '/' {
			hostAndPath = hostAndPath[:hostAndPathLen-1]
		}
		column[hostAndPath] = struct{}{}
	}

	return column
}

// IntersectCSV two csv files looking for maxCount intersections, writing
// records to write. Reads all of read2 into memory, so it should be the smaller
// file.
func IntersectCSV(read1, read2, write string, maxCount int) error {
	f1, err := os.Open(read1)
	if err != nil {
		return err
	}
	defer f1.Close()
	csv1 := csv.NewReader(f1)

	f2, err := os.Open(read2)
	if err != nil {
		return err
	}
	defer f2.Close()
	csv2 := csv.NewReader(f2)

	cL, err := csv2.ReadAll()
	if err != nil {
		return err
	}
	checkList := getColumn(cL, 0)

	if len(checkList) <= 0 {
		return errors.Errorf("checkList is len %d\n", len(checkList))
	}

	f3, err := os.Create(write)
	if err != nil {
		return err
	}
	defer f3.Close()
	writeCSV := csv.NewWriter(f3)

	writes := 0
	rowCount := 0

	for writes < maxCount {
		row, err := csv1.Read()
		rowCount++
		if err != nil {
			return err
		}
		_, ok := checkList[row[1]]

		if ok {
			writeCSV.Write(row)
			writes++
		}
	}

	writeCSV.Flush()
	infoLogger.Printf("Read %d records of %s to get intersection of size %d\n", rowCount, read1, maxCount)

	return nil
}
