package pointstores

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	elastic "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
)

// ElasticAdapter is a point store implementation for Elasticsearch
type ElasticAdapter struct {
	client *elastic.Client
}

// QResponse is used to facilitate parsing elasticsearch point query responses
type QResponse struct {
	Took     int  `json:"took"`
	TimedOut bool `json:"timed_out"`
	Hits     struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []struct {
			Index string `json:"_index"`
			ID    string `json:"_id"`
			P     Point  `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// NewElasticAdapter returns an initialized Elasticsearch point store
func NewElasticAdapter(sp config.SeriesParams) (*ElasticAdapter, error) {
	cfg := elastic.Config{
		Addresses: strings.Split(sp.StoreParams["URLs"].(string), ","),
		Username:  sp.StoreUser,
		Password:  sp.StorePass,
	}
	client, err := elastic.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &ElasticAdapter{client}, nil
}

type mappingProps struct {
	Type  string `json:"type"`
	Index bool   `json:"index"`
}

// AddPoint upserts a measurement into the index of a given series
func (ea ElasticAdapter) AddPoint(name string, p Point) error {
	index := prefix + cleanIndex(name)
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: p.ID(),
		Body:       strings.NewReader(string(data)),
	}
	logger.Trace("Indexing document with this content: " + string(data))

	res, err := req.Do(context.Background(), ea.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		if res.StatusCode == 404 {
			err = ea.AddSeries(name, p, 90)
			if err != nil {
				return err
			}
			return ea.AddPoint(name, p)
		}
		return esToErr("indexing document", res.Status())
	}
	// Deserialize the response into a map.
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return err
	}
	// Print the response status and indexed document version.
	logger.Trace(fmt.Sprintf("[%s] %s; version=%d", res.Status(), r["result"], int(r["_version"].(float64))))

	return nil
}

// AddSeries creates and configures a new index in Elasticsearch to hold a time series
func (ea ElasticAdapter) AddSeries(name string, sample Point, retentionDays int) error {
	index := prefix + cleanIndex(name)
	props := make(map[string]mappingProps, 1+len(sample.Labels)+len(sample.Values))

	props["@timestamp"] = mappingProps{"date", true}
	for label := range sample.Labels {
		props[label] = mappingProps{"keyword", true}
	}
	for value := range sample.Values {
		props[value] = mappingProps{"float", false}
	}

	jProps, err := json.Marshal(props)
	mapping := `{"mappings":{"date_detection": false, "properties":` + string(jProps) + "}}"
	logger.Info("Creating new index for series " + name + " with this mapping: " + mapping)
	res, err := ea.client.Indices.Create(index, ea.client.Indices.Create.WithBody(strings.NewReader(mapping)))
	if err != nil {
		return err
	}
	if res.IsError() {
		return esToErr("creating index", res.Status())
	}
	return nil
}

// DeleteSeries removes the index used to store a series
func (ea ElasticAdapter) DeleteSeries(name string) error {
	index := prefix + cleanIndex(name)
	res, err := ea.client.Indices.Delete([]string{index})
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

// Exists returns true if an index for the specified series is present in elasticsearch, false if not or in case of
// error (make sure to check if error is nil before looking at the boolean)
func (ea ElasticAdapter) Exists(name string) (bool, error) {
	index := prefix + cleanIndex(name)
	req := esapi.IndicesExistsRequest{
		Index: []string{index},
	}
	res, err := req.Do(context.Background(), ea.client)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.IsError() {
		if res.StatusCode == 404 {
			return false, nil
		}
		return false, esToErr("checking if index exists", res.Status())
	}
	return true, nil
}

// GetCount retrieves the number of points recorded for the given series with the specified labels (returns 0 if the
// series doesn't exist)
func (ea ElasticAdapter) GetCount(name string, labels map[string]string) (int, error) {
	index := prefix + cleanIndex(name)
	res, err := ea.client.Count(
		ea.client.Count.WithContext(context.Background()),
		ea.client.Count.WithIndex(index),
	)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.IsError() {
		if res.StatusCode == 404 {
			return 0, nil
		}
		buf := new(strings.Builder)
		_, err = io.Copy(buf, res.Body)
		if err != nil {
			return 0, err
		}
		return 0, esToErr("performing count query", buf.String())
	}
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return 0, err
	}

	return int(r["count"].(float64)), nil
}

// GetLatest retrieves the most recent value of the series with the specified labels
func (ea ElasticAdapter) GetLatest(name string, labels map[string]string) (Point, error) {
	index := prefix + cleanIndex(name)
	stmt := `{"sort": [{"@timestamp": {"order": "desc"}}],"size": 1}`
	res, err := ea.query(index, stmt)
	if err != nil {
		return Point{}, err
	}
	if len(res.Hits.Hits) < 1 {
		return Point{}, errors.New("No points found")
	}
	return res.Hits.Hits[0].P, nil
}

// GetLastN retrieves the last N points for the given series with the specified labels
func (ea ElasticAdapter) GetLastN(name string, labels map[string]string, n int) ([]Point, error) {
	index := prefix + cleanIndex(name)
	stmt := `{"sort": [{"@timestamp": {"order": "desc"}}],"size": ` + strconv.Itoa(n) + `}`
	res, err := ea.query(index, stmt)
	if err != nil {
		return nil, err
	}
	points := []Point{}
	for _, hit := range res.Hits.Hits {
		points = append(points, hit.P)
	}

	return points, nil
}

// ListSeries as its name implies, returns a list of all the series that are available in elasticsearch
func (ea ElasticAdapter) ListSeries() ([]types.BriefSeries, error) {
	req := esapi.CatIndicesRequest{
		Index:  []string{prefix + "*"},
		Format: "json",
		H:      []string{"index", "docs.count"},
	}
	res, err := req.Do(context.Background(), ea.client)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, esToErr("listing indices", res.Status())
	}
	logger.Trace(res.String())
	var r []map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}
	var series []types.BriefSeries
	for _, s := range r {
		count := 0
		if s["docs.count"] != nil {
			count, err = strconv.Atoi(s["docs.count"].(string))
			if err != nil {
				return nil, err
			}
		}
		series = append(series, types.BriefSeries{Name: s["index"].(string)[len(prefix):], Count: count})
	}

	return series, nil
}

// LoadTestSet loads a set of points from a single file for testing (not part of the standard PointStore interface)
func (ea ElasticAdapter) LoadTestSet(name, path string) error {
	ps := FileAdapter{Path: "."}
	points, err := ps.LoadTestSet(path)
	if err != nil {
		return err
	}
	for _, point := range points {
		err = ea.AddPoint(name, point)
		if err != nil {
			return err
		}
	}
	return nil
}

func esToErr(context, err string) error {
	return errors.New("Error encountered while " + context + ": " + err)
}

func (ea ElasticAdapter) query(index, stmt string) (QResponse, error) {
	logger.Trace("Executing query " + stmt + " for index " + index)
	res, err := ea.client.Search(
		ea.client.Search.WithContext(context.Background()),
		ea.client.Search.WithIndex(index),
		ea.client.Search.WithBody(strings.NewReader(stmt)),
		ea.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return QResponse{}, err
	}
	defer res.Body.Close()
	if res.IsError() {
		buf := new(strings.Builder)
		_, err = io.Copy(buf, res.Body)
		if err != nil {
			return QResponse{}, err
		}
		return QResponse{}, esToErr("performing this ("+stmt+") query", buf.String())
	}

	var r QResponse
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return QResponse{}, err
	}

	if len(r.Hits.Hits) <= 3 {
		logger.Trace("Response from elasticsearch: " + res.String())
	} else {
		logger.Trace(
			fmt.Sprintf(
				"Response from elasticsearch too big to print in full, status: %d hits: %d",
				res.StatusCode,
				len(r.Hits.Hits),
			),
		)
	}

	return r, nil
}

func cleanIndex(name string) string {
	res := strings.ToLower(name)
	return strings.ReplaceAll(res, ":", "_")
}
