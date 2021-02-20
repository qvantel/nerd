package pointstores

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/qvantel/nerd/internal/config"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testElasticStore PointStore

func getTestStore(url string) (PointStore, error) {
	var storeParams map[string]interface{}
	json.Unmarshal([]byte(`{"URLs": "`+url+`"}`), &storeParams)
	conf := config.Config{
		Series: config.SeriesParams{
			StoreType:   config.ElasticsearchSeriesStore,
			StoreParams: storeParams,
		},
	}
	return New(conf)
}

func initTest(name string) error {
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

	err := testElasticStore.AddPoint(name, p1)
	if err != nil {
		return err
	}
	err = testElasticStore.AddPoint(name, p2)
	if err != nil {
		return err
	}
	// Pause for refresh
	time.Sleep(1 * time.Second)

	return nil
}

func startElastic(ctx context.Context) (elastic testcontainers.Container, url string, err error) {
	req := testcontainers.ContainerRequest{
		Env: map[string]string{
			"discovery.type":           "single-node",
			"action.auto_create_index": ".watches,.triggered_watches,.watcher-history-*",
		},
		Image:        "elasticsearch:7.10.1",
		ExposedPorts: []string{"9200/tcp"},
		WaitingFor:   wait.ForHTTP("/").WithPort("9200/tcp"),
	}
	elastic, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}
	endpoint, err := elastic.Endpoint(ctx, "")
	if err != nil {
		return nil, "", err
	}
	return elastic, "http://" + endpoint, nil
}

func TestExists(t *testing.T) {
	found, err := testElasticStore.Exists(t.Name() + "-should-not-exist")
	if err != nil {
		t.Fatalf("Failed to check if series exists (%s)", err.Error())
	}
	if found {
		t.Errorf("Series doesn't exist so the Exists method should return false, it instead returned true")
	}
	sec := int64(777808800)
	p1 := Point{
		Labels:    map[string]string{"env": "test"},
		Values:    map[string]float32{"subs": 12000, "events": 5634746, "size": 50},
		TimeStamp: sec,
	}
	err = testElasticStore.AddSeries(t.Name(), p1, 0)
	if err != nil {
		t.Fatalf("Failed to add series to store (%s)", err.Error())
	}
	defer testElasticStore.DeleteSeries(t.Name())
	found, err = testElasticStore.Exists(t.Name())
	if err != nil {
		t.Fatalf("Failed to check if index exists in store (%s)", err.Error())
	}
	if !found {
		t.Errorf("Series exists so the Exists method should return true, it instead returned false")
	}
}

func TestGetLatest(t *testing.T) {
	err := initTest(t.Name())
	if err != nil {
		t.Fatalf("Failed to initialize point store (%s)", err.Error())
	}
	defer testElasticStore.DeleteSeries(t.Name())

	p2 := Point{
		Labels:    map[string]string{"env": "test"},
		Values:    map[string]float32{"subs": 12010, "events": 5634746, "size": 51},
		TimeStamp: int64(777808800) + 60,
	}

	p, err := testElasticStore.GetLatest(t.Name(), map[string]string{})
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

func TestGetLastN(t *testing.T) {
	err := initTest(t.Name())
	if err != nil {
		t.Fatalf("Failed to initialize point store (%s)", err.Error())
	}
	defer testElasticStore.DeleteSeries(t.Name())

	points, err := testElasticStore.GetLastN(t.Name(), map[string]string{}, 4)
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

func TestGetCount(t *testing.T) {
	err := initTest(t.Name())
	if err != nil {
		t.Fatalf("Failed to initialize point store (%s)", err.Error())
	}
	defer testElasticStore.DeleteSeries(t.Name())

	res, err := testElasticStore.GetCount(t.Name(), nil)
	if err != nil {
		t.Fatalf("Failed to get series count (%s)", err.Error())
	}
	if res != 2 {
		t.Errorf("Wrong count, expected 2 got %d", res)
	}
}

func TestListSeries(t *testing.T) {
	err := initTest(t.Name())
	if err != nil {
		t.Fatalf("Failed to initialize point store (%s)", err.Error())
	}
	defer testElasticStore.DeleteSeries(t.Name())

	res, err := testElasticStore.ListSeries()
	if err != nil {
		t.Fatalf("Failed to get series list (%s)", err.Error())
	}
	if len(res) < 1 {
		t.Fatalf("Expected to retrieve at least one result")
	}

	found := false
	for _, series := range res {
		if series.Name == cleanIndex(t.Name()) {
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

func TestLoadTestSet(t *testing.T) {
	found, err := testElasticStore.Exists(t.Name())
	if err != nil {
		t.Fatalf("Failed to check if series exists (%s)", err.Error())
	}
	if found {
		t.Skip("Test set is already present, skipping to avoid overwhelming the test instance")
	}
	ea := testElasticStore.(*ElasticAdapter)

	err = ea.LoadTestSet(t.Name(), "test/normalization_test_data.txt")
	if os.IsNotExist(err) {
		err = ea.LoadTestSet(t.Name(), "../../../test/normalization_test_data.txt")
	}
	if err != nil {
		t.Fatalf("Failed to load test data (%s)", err.Error())
	}
}

func TestMain(m *testing.M) {
	// Setup
	ctx := context.Background()
	elastic, url, err := startElastic(ctx)
	if err != nil {
		fmt.Printf("Error starting test Elasticsearch container (%s)", err.Error())
		os.Exit(1)
	}
	testElasticStore, err = getTestStore(url)
	if err != nil {
		fmt.Printf("Failed to get point store (%s)", err.Error())
		os.Exit(1)
	}
	// Run
	code := m.Run()
	// Teardown
	if err == nil {
		elastic.Terminate(ctx)
	}
	os.Exit(code)
}
