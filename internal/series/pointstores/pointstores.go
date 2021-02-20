// Package pointstores contains the implementation of all the supported storage adapters for the series
package pointstores

import (
	"errors"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
)

const prefix string = "nerd-"

// PointStore is an abstraction over the storage service that will be used to store the measurements taken from envs
type PointStore interface {
	// Adds a point to a series, should create it if it doesn't exist (calling AddSeries)
	AddPoint(name string, p Point) error
	// Create a new series
	AddSeries(name string, sample Point, retentionDays int) error
	// Delete a series
	DeleteSeries(name string) error
	Exists(name string) (bool, error)
	GetCount(name string, labels map[string]string) (int, error)
	// Gets the current value of the series
	GetLatest(name string, labels map[string]string) (Point, error)
	GetLastN(name string, labels map[string]string, n int) ([]Point, error)
	// Get list of available series
	ListSeries() ([]types.BriefSeries, error)
}

// New returns an initialized point store of the type specified in the configuration
func New(conf config.Config) (PointStore, error) {
	switch conf.Series.StoreType {
	case config.FileSeriesStore:
		return NewFileAdapter(conf.Series.StoreParams)
	case config.ElasticsearchSeriesStore:
		return NewElasticAdapter(conf.Series)
	default:
		return nil, errors.New(conf.Series.StoreType + " is not a valid point store type")
	}
}
