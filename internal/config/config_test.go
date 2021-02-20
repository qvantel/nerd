package config

import (
	"testing"
)

func TestGetenv(t *testing.T) {
	got := Getenv("VAR_THAT_DOES_NOT_EXIST", "default value")
	if got != "default value" {
		t.Errorf("Getenv(\"VAR_THAT_DOES_NOT_EXIST\", \"default value\") = %s; want default value", got)
	}
}

func TestMLParamsCheck(t *testing.T) {
	valid := MLParams{
		Generations: 5,
		MaxEpoch:    1000,
		MaxHLayers:  5,
		MinHLayers:  1,
		StoreType:   FileParamStore,
		StoreParams: map[string]interface{}{"Path": "."},
		TestSet:     0.4,
		Tolerance:   0.1,
		Variations:  6,
	}
	gens, maxE, maxL, minL, storeT, testS, vars := valid, valid, valid, valid, valid, valid, valid

	err := valid.Check()
	if err != nil {
		t.Errorf("MLParams check returned an error for valid params (%s)", err.Error())
	}
	gens.Generations = 0
	if gens.Check() == nil {
		t.Error("A generations value of less than 1 didn't return an error when checked")
	}
	maxE.MaxEpoch = 0
	if maxE.Check() == nil {
		t.Error("A max epoch of 0 didn't return an error when checked")
	}
	maxL.MaxHLayers = 0
	if maxL.Check() == nil {
		t.Error("A max hidden layers value lower than the min didn't return an error when checked")
	}
	minL.MinHLayers = 0
	if minL.Check() == nil {
		t.Error("A min hidden layers value lower than 1 didn't return an error when checked")
	}
	storeT.StoreType = "invalid-type"
	if storeT.Check() == nil {
		t.Error("An invalid param store type didn't return an error when checked")
	}
	testS.TestSet = -1
	if testS.Check() == nil {
		t.Error("A negative test set didn't return an error when checked")
	}
	testS.TestSet = 1
	if testS.Check() == nil {
		t.Error("A test set of 1 (no patterns left for training) didn't return an error when checked")
	}
	vars.Variations = 3
	if vars.Check() == nil {
		t.Error("A variations value lower than 4 didn't return an error when checked")
	}
}

func TestSeriesParamsCheck(t *testing.T) {
	valid := SeriesParams{
		FailLimit: 5,
		Source: Kafka{
			Brokers: []string{"localhost:9092"},
			GroupID: "nerd",
			Topic:   "nerd-events",
		},
		StoreType:   FileSeriesStore,
		StoreParams: map[string]interface{}{"Path": "."},
	}
	storeT := valid

	err := valid.Check()
	if err != nil {
		t.Errorf("SeriesParams check returned an error for valid params (%s)", err.Error())
	}
	storeT.StoreType = "invalid-type"
	if storeT.Check() == nil {
		t.Error("An invalid point store type didn't return an error when checked")
	}
}

func TestNew(t *testing.T) {
	_, err := New()
	if err != nil {
		t.Fatalf("Failed to get config (%s)", err.Error())
	}
}
