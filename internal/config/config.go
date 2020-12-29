// Package config centralizes the parsing of application configuration
package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
)

// Kafka holds the necessary configuration to set up the connection to a Kafka cluster
type Kafka struct {
	Brokers []string
	GroupID string
	Topic   string
}

// LoggerParams holds the necessary configuration to initialize the logger
type LoggerParams struct {
	ArtifactID  string
	Level       string
	ServiceName string
}

// MLParams holds the parameters that determine how the ML package will behave and how it will store its data
type MLParams struct {
	ActivationFunc string
	Alpha          float32
	HLayers        int
	Net            string
	MaxEpoch       int
	StoreType      string
	StoreParams    map[string]interface{}
	TestSet        float32
	Tolerance      float32
}

// SeriesParams holds the parameters that determine how the series package will behave and how it will store its data
type SeriesParams struct {
	FailLimit   int
	Source      Kafka
	StoreType   string
	StoreParams map[string]interface{}
	StorePass   string
	StoreUser   string
}

// Config holds all the configuration for the app
type Config struct {
	AppVersion string
	Logger     LoggerParams
	ML         MLParams
	Series     SeriesParams
}

// New generates a Config object populated with values from the environment
func New() (*Config, error) {
	conf := Config{}

	// Take care of logging params first in case the app has to report a config related error
	conf.AppVersion = Getenv("VERSION", "unknown")
	conf.Logger.Level = Getenv("LOG_LEVEL", "INFO")
	conf.Logger.ArtifactID = Getenv("MARATHON_APP_DOCKER_IMAGE", "qvantel/nerd:"+conf.AppVersion+"?")
	conf.Logger.ServiceName = Getenv("SERVICE_5400_NAME", Getenv("SERVICE_NAME", "nerd"))

	// ML params
	conf.ML.ActivationFunc = Getenv("ML_ACT_FUNC", "bipolar-sigmoid")
	lr, err := strconv.ParseFloat(Getenv("ML_ALPHA", "0.05"), 32)
	if err != nil {
		return &conf, err
	}
	conf.ML.Alpha = float32(lr)
	conf.ML.HLayers, err = strconv.Atoi(Getenv("ML_HLAYERS", "1"))
	if err != nil {
		return &conf, err
	}
	conf.ML.Net = Getenv("ML_NET", "mlp")
	if conf.ML.Net != "mlp" {
		return &conf, errors.New(conf.ML.Net + " is not a valid net type")
	}
	conf.ML.MaxEpoch, err = strconv.Atoi(Getenv("ML_MAX_EPOCH", "1000"))
	if err != nil {
		return &conf, err
	}
	conf.ML.StoreType = Getenv("ML_STORE_TYPE", "file")
	defMLStoreParams := `{"Path": "."}`
	redis := os.Getenv("SD_REDIS")
	if redis != "" {
		defMLStoreParams = `{"URL": "` + redis + `"}`
	}
	err = json.Unmarshal([]byte(Getenv("ML_STORE_PARAMS", defMLStoreParams)), &conf.ML.StoreParams)
	if err != nil {
		return &conf, err
	}
	ts, err := strconv.ParseFloat(Getenv("ML_TEST_SET", "0.4"), 32)
	if err != nil {
		return &conf, err
	}
	if ts < 0 || ts >= 1 {
		return &conf, errors.New("test set must be between 0 (included) and 1 (not included)")
	}
	conf.ML.TestSet = float32(ts)
	tolerance, err := strconv.ParseFloat(Getenv("ML_TOLERANCE", "0.1"), 32)
	if err != nil {
		return &conf, err
	}
	conf.ML.Tolerance = float32(tolerance)

	// Series params
	conf.Series.FailLimit, err = strconv.Atoi(Getenv("SERIES_FAIL_LIMIT", "5"))
	if err != nil {
		return &conf, err
	}
	brokers := os.Getenv("SD_KAFKA")
	if brokers == "" {
		return &conf, errors.New("no value found for required variable SD_KAFKA")
	}
	conf.Series.Source = Kafka{
		Brokers: strings.Split(brokers, ","),
		GroupID: Getenv("SERIES_KAFKA_GROUP", "nerd"),
		Topic:   Getenv("SERIES_KAFKA_TOPIC", "nerd-events"),
	}
	conf.Series.StoreType = Getenv("SERIES_STORE_TYPE", "file")
	defSeriesStoreParams := `{"Path": "."}`
	esNodes := os.Getenv("SD_ELASTICSEARCH")
	if esNodes != "" {
		defSeriesStoreParams = `{"URLs": "` + esNodes + `"}`
	}
	err = json.Unmarshal([]byte(Getenv("SERIES_STORE_PARAMS", defSeriesStoreParams)), &conf.Series.StoreParams)
	if err != nil {
		return &conf, err
	}
	conf.Series.StorePass = os.Getenv("SERIES_STORE_PASS")
	conf.Series.StoreUser = os.Getenv("SERIES_STORE_USER")

	return &conf, nil
}

// Getenv is useful for retrieving the value of an env var with a default
func Getenv(env, fallback string) string {
	value := os.Getenv(env)
	if value == "" {
		return fallback
	}
	return value
}
