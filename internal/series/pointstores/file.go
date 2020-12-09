package pointstores

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/qvantel/nerd/api/types"
)

// FileAdapter is a point store implementation that uses the filesystem. Its main purpose is to facilitate
// testing, given its low performance it is strongly discouraged for production use
type FileAdapter struct {
	Path string
}

// NewFileAdapter returns an initialized file point store object
func NewFileAdapter(conf map[string]interface{}) (*FileAdapter, error) {
	return &FileAdapter{Path: conf["Path"].(string)}, nil
}

// AddPoint creates a new file with the JSON representation of the point in the subdirectory that corresponds to the
// given series
func (fa FileAdapter) AddPoint(name string, p Point) error {
	dir := prefix + cleanDir(name)
	series := fa.Path + "/" + dir
	if _, err := os.Stat(series); os.IsNotExist(err) {
		err = fa.AddSeries(name, p, 90)
		if err != nil {
			return err
		}
	}
	f, err := os.Create(fa.Path + "/" + dir + "/" + p.ID())
	jPoint, err := json.Marshal(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(string(jPoint))
	if err != nil {
		return err
	}
	f.Sync()
	ts := time.Unix(p.TimeStamp, 0)
	os.Chtimes(fa.Path+"/"+dir+"/"+p.ID(), ts, ts)
	return nil
}

// AddSeries creates a directory to hold a time series
func (fa FileAdapter) AddSeries(name string, sample Point, retentionDays int) error {
	dir := prefix + cleanDir(name)
	err := os.Mkdir(fa.Path+"/"+dir, 0755)
	return err
}

// DeleteSeries removes the subdirectory used to store a series
func (fa FileAdapter) DeleteSeries(name string) error {
	dir := prefix + cleanDir(name)
	return os.RemoveAll(fa.Path + "/" + dir)
}

// Exists returns true if a directory is present for the specified series, false if not or in case of error
func (fa FileAdapter) Exists(name string) (bool, error) {
	dir := prefix + cleanDir(name)
	series := fa.Path + "/" + dir
	_, err := os.Stat(series)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetCount retrieves the number of points recorded for the given series (labels are ignored in this adapter for speed)
// (returns 0 if the series doesn't exist)
func (fa FileAdapter) GetCount(name string, labels map[string]string) (int, error) {
	dir := prefix + cleanDir(name)
	files, err := ioutil.ReadDir(fa.Path + "/" + dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// GetLatest returns the most recent value of the series by looking for the most recent file in its subdir
func (fa FileAdapter) GetLatest(name string, labels map[string]string) (Point, error) {
	points, err := fa.GetLastN(name, labels, 1)
	if err != nil {
		return Point{}, err
	}
	return points[0], nil
}

// GetLastN returns the last n points for the given series
func (fa FileAdapter) GetLastN(name string, labels map[string]string, n int) ([]Point, error) {
	dir := prefix + cleanDir(name)

	files, err := ioutil.ReadDir(fa.Path + "/" + dir)
	if err != nil {
		return nil, err
	}
	if len(files) < n {
		n = len(files)
	}
	sort.SliceStable(files, func(i, j int) bool {
		return files[j].ModTime().Before(files[i].ModTime())
	})

	var points []Point
	for _, file := range files[:n] {
		dat, err := ioutil.ReadFile(fa.Path + "/" + dir + "/" + file.Name())
		if err != nil {
			return nil, err
		}
		var p Point
		err = json.Unmarshal(dat, &p)
		if err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	for _, point := range points {
		fmt.Printf(" - %s %d\n", point.ID(), point.TimeStamp)
	}

	return points, nil
}

// ListSeries returns a list of all the available series in the configured directory
func (fa FileAdapter) ListSeries() ([]types.BriefSeries, error) {
	files, err := ioutil.ReadDir(fa.Path)
	if err != nil {
		return nil, err
	}
	var series []types.BriefSeries
	for _, file := range files {
		if !file.IsDir() || file.Name()[:len(prefix)] != prefix {
			continue
		}
		name := file.Name()[len(prefix):]
		count, err := fa.GetCount(name, nil)
		if err != nil {
			return nil, err
		}
		series = append(series, types.BriefSeries{
			Name:  name,
			Count: count,
		})
	}

	return series, nil
}

// LoadTestSet loads a set of points from a single file for testing (not part of the standard PointStore interface)
func (fa FileAdapter) LoadTestSet(name string) ([]Point, error) {
	file, err := os.Open(fa.Path + "/" + name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	t := int64(777808800)
	var points []Point
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		values := strings.Split(scanner.Text(), " ")
		vmap := map[string]float32{}
		for i, rValue := range values {
			value, err := strconv.ParseFloat(rValue, 32)
			if err != nil {
				return nil, err
			}
			vmap["value-"+strconv.Itoa(i)] = float32(value)
		}
		points = append(points, Point{Values: vmap, TimeStamp: t})
		t++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func cleanDir(name string) string {
	res := strings.ToLower(name)
	return strings.ReplaceAll(res, ":", "_")
}
