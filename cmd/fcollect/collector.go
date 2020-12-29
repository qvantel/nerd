package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/qvantel/nerd/api/types"
)

// FileCollector is a nerd collector implementation for extracting points from lines in a file
type FileCollector struct {
	err     error
	Headers bool
	InN     int
	Out     chan types.CategorizedPoint
	Path    string
	Sep     string
}

// NewFileCollector creates a new file collector instance
func NewFileCollector(headers bool, inN int, out chan types.CategorizedPoint, path, sep string) *FileCollector {
	return &FileCollector{
		Headers: headers,
		InN:     inN,
		Out:     out,
		Path:    path,
		Sep:     sep,
	}
}

// Collect reads the file at the configured path and returns categorized points through the collector's channel
func (fc *FileCollector) Collect() {
	file, err := os.Open(fc.Path)
	if err != nil {
		fc.err = err
		close(fc.Out)
		return
	}
	defer file.Close()

	// Get file mod time for autogenerating the point timestamps
	info, err := os.Stat(fc.Path)
	if err != nil {
		fc.err = err
		close(fc.Out)
		return
	}
	ts := info.ModTime().Unix()

	scanner := bufio.NewScanner(file)

	labels := []string{}

	for line := 0; scanner.Scan(); line++ {
		values := strings.Split(scanner.Text(), fc.Sep)
		if line == 0 && fc.Headers {
			labels = values
			continue
		}
		if line == 0 {
			for i := range values {
				labels = append(labels, "value-"+strconv.Itoa(i))
			}
		}
		inputs := make(map[string]float32, fc.InN)
		outputs := make(map[string]float32, len(labels)-fc.InN)
		for i := range values {
			value, err := strconv.ParseFloat(values[i], 32)
			if err != nil {
				fmt.Println("WARNING: Encountered incorrectly formatted float on line " + strconv.Itoa(line+1))
				break
			}
			if i < fc.InN {
				inputs[labels[i]] = float32(value)
			} else {
				outputs[labels[i]] = float32(value)
			}
		}
		if len(inputs)+len(outputs) != len(labels) {
			continue
		}
		fc.Out <- types.CategorizedPoint{
			Inputs:    inputs,
			Outputs:   outputs,
			TimeStamp: ts,
		}
		ts++
	}
	if err := scanner.Err(); err != nil {
		fc.err = err
	}
	close(fc.Out)
}

// Err returns the last recorded error during collection or nil if none were encountered
func (fc *FileCollector) Err() error {
	return fc.err
}
