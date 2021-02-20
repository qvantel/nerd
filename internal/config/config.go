// Package config centralizes the parsing of application configuration
package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
)

// Supported network parameter store types
const (
	FileParamStore  = "file"
	RedisParamStore = "redis"
)

var paramStoreTypes = []string{FileParamStore, RedisParamStore}

// Supported series store types
const (
	FileSeriesStore          = FileParamStore
	ElasticsearchSeriesStore = "elasticsearch"
)

var seriesStoreTypes = []string{FileSeriesStore, ElasticsearchSeriesStore}

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
	Generations int // Number of cycles to run the genetic algorithm for in search of the optimal net params
	MaxEpoch    int
	MaxHLayers  int // Maximum starting number of hidden layers (the genetic algorithm can surpass it)
	MinHLayers  int // Minimum starting number of hidden layers (the genetic algorithm can go down to 1)
	StoreType   string
	StoreParams map[string]interface{}
	TestSet     float32
	Tolerance   float32
	Variations  int // Number of different network configs to evaluate in each generation of the genetic algorithm
}

// Check will return an error if any of the machine learning params have semantically incorrect values
func (mlParams *MLParams) Check() error {
	if mlParams.Generations < 1 {
		return errors.New("at least one generation is needed to allow for networks to be trained")
	}
	if mlParams.MaxEpoch < 1 {
		return errors.New("a max epoch bellow 1 would mean that networks wouldn't be trained at all")
	}
	if mlParams.MinHLayers < 1 {
		return errors.New("there should be at least 1 hidden layer in all nets")
	}
	if mlParams.MaxHLayers < mlParams.MinHLayers {
		return errors.New("the maximum number of hidden layers must be equal or greater than the minimum")
	}
	if !Present(paramStoreTypes, mlParams.StoreType) {
		return errors.New(mlParams.StoreType + " is not a valid param store type")
	}
	if mlParams.TestSet < 0 || mlParams.TestSet >= 1 {
		return errors.New("test set must be between 0 (included) and 1 (not included)")
	}
	if mlParams.Variations < 4 {
		return errors.New("at least four variations are needed")
	}
	return nil
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

// Check will return an error if any of the series params have semantically incorrect values
func (seriesParams *SeriesParams) Check() error {
	if !Present(seriesStoreTypes, seriesParams.StoreType) {
		return errors.New(seriesParams.StoreType + " is not a valid point store type")
	}
	return nil
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
	err := loadMLParams(&conf)
	if err != nil {
		return &conf, err
	}
	err = conf.ML.Check()
	if err != nil {
		return &conf, err
	}

	// Series params
	err = loadSeriesParams(&conf)
	if err != nil {
		return &conf, err
	}
	err = conf.Series.Check()
	if err != nil {
		return &conf, err
	}

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

// loadMLParams parses the part of the config that determines the behavior of the machine learning logic
func loadMLParams(conf *Config) (err error) {
	conf.ML.Generations, err = strconv.Atoi(Getenv("ML_GENS", "5"))
	if err != nil {
		return err
	}
	conf.ML.MinHLayers, err = strconv.Atoi(Getenv("ML_MIN_HLAYERS", "1"))
	if err != nil {
		return err
	}
	conf.ML.MaxHLayers, err = strconv.Atoi(Getenv("ML_MAX_HLAYERS", "5"))
	if err != nil {
		return err
	}
	conf.ML.MaxEpoch, err = strconv.Atoi(Getenv("ML_MAX_EPOCH", "1000"))
	if err != nil {
		return err
	}
	conf.ML.StoreType = Getenv("ML_STORE_TYPE", FileParamStore)
	defNPSParams := `{"Path": "."}`
	redis := os.Getenv("SD_REDIS")
	if redis != "" {
		defNPSParams = `{"URL": "` + redis + `"}`
	}
	err = json.Unmarshal([]byte(Getenv("ML_STORE_PARAMS", defNPSParams)), &conf.ML.StoreParams)
	if err != nil {
		return err
	}
	ts, err := strconv.ParseFloat(Getenv("ML_TEST_SET", "0.4"), 32)
	if err != nil {
		return err
	}
	conf.ML.TestSet = float32(ts)
	tolerance, err := strconv.ParseFloat(Getenv("ML_TOLERANCE", "0.1"), 32)
	if err != nil {
		return err
	}
	conf.ML.Tolerance = float32(tolerance)
	conf.ML.Variations, err = strconv.Atoi(Getenv("ML_VARS", "6"))
	if err != nil {
		return err
	}
	return nil
}

// loadSeriesParams parses the part of the config that determines the behavior of the series logic
func loadSeriesParams(conf *Config) (err error) {
	conf.Series.FailLimit, err = strconv.Atoi(Getenv("SERIES_FAIL_LIMIT", "5"))
	if err != nil {
		return err
	}
	brokers := os.Getenv("SD_KAFKA")
	if brokers != "" {
		conf.Series.Source = Kafka{
			Brokers: strings.Split(brokers, ","),
			GroupID: Getenv("SERIES_KAFKA_GROUP", "nerd"),
			Topic:   Getenv("SERIES_KAFKA_TOPIC", "nerd-events"),
		}
	}
	conf.Series.StoreType = Getenv("SERIES_STORE_TYPE", FileSeriesStore)
	defSeriesStoreParams := `{"Path": "."}`
	esNodes := os.Getenv("SD_ELASTICSEARCH")
	if esNodes != "" {
		defSeriesStoreParams = `{"URLs": "` + esNodes + `"}`
	}
	err = json.Unmarshal([]byte(Getenv("SERIES_STORE_PARAMS", defSeriesStoreParams)), &conf.Series.StoreParams)
	if err != nil {
		return err
	}
	conf.Series.StorePass = os.Getenv("SERIES_STORE_PASS")
	conf.Series.StoreUser = os.Getenv("SERIES_STORE_USER")
	return nil
}

// Present returns true if the given element is in the given list, false if not
func Present(list []string, element string) bool {
	for _, e := range list {
		if e == element {
			return true
		}
	}
	return false
}
