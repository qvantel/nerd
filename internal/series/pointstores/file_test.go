package pointstores

import (
	"encoding/json"
	"testing"

	"github.com/qvantel/nerd/internal/config"
)

func getTestFileStore() (PointStore, error) {
	var storeParams map[string]interface{}
	json.Unmarshal([]byte(`{"Path": "."}`), &storeParams)
	conf := config.Config{
		Series: config.SeriesParams{
			StoreType:   config.FileSeriesStore,
			StoreParams: storeParams,
		},
	}
	return New(conf)
}

func initFileStoreTest(name string) (PointStore, error) {
	ps, err := getTestFileStore()
	if err != nil {
		return FileAdapter{}, err
	}

	sec := int64(777808800)
	p1 := Point{
		Labels:    map[string]string{"env": "test"},
		Values:    map[string]float32{"subs": 12000, "events": 5634746, "size": 50},
		TimeStamp: sec,
	}
	p2 := Point{
		Labels:    map[string]string{"env": "test"},
		Values:    map[string]float32{"subs": 12010, "events": 5634746, "size": 51},
		TimeStamp: sec + 60,
	}

	err = ps.AddPoint(name, p1)
	if err != nil {
		return FileAdapter{}, err
	}
	err = ps.AddPoint(name, p2)
	if err != nil {
		return FileAdapter{}, err
	}

	return ps, nil
}

func TestGetLatestFile(t *testing.T) {
	ps, err := initFileStoreTest(t.Name())
	if err != nil {
		t.Fatalf("Failed to initialize point store (%s)", err.Error())
	}
	defer ps.DeleteSeries(t.Name())

	p2 := Point{
		Labels:    map[string]string{"env": "test"},
		Values:    map[string]float32{"subs": 12010, "events": 5634746, "size": 51},
		TimeStamp: int64(777808800) + 60,
	}

	p, err := ps.GetLatest(t.Name(), map[string]string{})
	if err != nil {
		t.Fatalf("Failed to get latest point from store (%s)", err.Error())
	}
	jPoint, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("The custom JSON marshal method shouldn't fail to convert a known valid point (%s)", err.Error())
	}
	if p.ID() != p2.ID() {
		t.Errorf("Point does not match the latest in the series, expected %s, got %s", p2.ID(), jPoint)
	}
}

func TestGetLastNFile(t *testing.T) {
	ps, err := initFileStoreTest(t.Name())
	if err != nil {
		t.Fatalf("Failed to initialize point store (%s)", err.Error())
	}
	defer ps.DeleteSeries(t.Name())

	points, err := ps.GetLastN(t.Name(), map[string]string{}, 4)
	if err != nil {
		t.Fatalf("Failed to get latest 4 points from store (%s)", err.Error())
	}
	if len(points) != 2 {
		t.Errorf("Number of points doesn't match current count, expected 2, got %d", len(points))
	}
	if points[1].TimeStamp != int64(777808800) {
		t.Errorf("Points should be ordered by timestamp (desc), expected 777808800 as the timestamp of the second point, got %d", points[1].TimeStamp)
	}
}

func TestListSeriesDirs(t *testing.T) {
	ps, err := initFileStoreTest(t.Name())
	if err != nil {
		t.Fatalf("Failed to initialize point store (%s)", err.Error())
	}
	defer ps.DeleteSeries(t.Name())

	res, err := ps.ListSeries()
	if err != nil {
		t.Fatalf("Failed to get series list (%s)", err.Error())
	}
	if len(res) < 1 {
		t.Fatalf("Expected to retrieve at least one result")
	}

	found := false
	for _, series := range res {
		if series.Name == cleanDir(t.Name()) {
			found = true
			if series.Count != 2 {
				t.Errorf("Test series has incorrect count, expected 2 got %d", series.Count)
			}
		}
	}
	if !found {
		t.Errorf("Test series is missing from the results array")
	}
}
