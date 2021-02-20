package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/nets/paramstores"
)

func TestEvaluate(t *testing.T) {
	// Build API
	conf := config.Config{
		ML: config.MLParams{
			StoreType:   config.FileParamStore,
			StoreParams: map[string]interface{}{"Path": "."},
		},
		Series: config.SeriesParams{
			StoreType:   config.FileSeriesStore,
			StoreParams: map[string]interface{}{"Path": "."},
		},
	}
	id := "test-evaluate-51e1890284194a8e4bb9923994e46cf59cfdd90d-89368e1d68015693ab48ee189d0632cb5d6edfb3-" + types.MultilayerPerceptron
	api, err := New(nil, conf)
	if err != nil {
		t.Fatalf("Failed to initialize API (%s)", err.Error())
	}

	// Create a network in a known state
	params := paramstores.MLPParams{
		ActivationFunc: types.BipolarSigmoid,
		Inputs:         []string{"subs", "events"},
		LearningRate:   0.25,
		Topology:       []int{2, 2, 1},
		Outputs:        []string{"size"},
		Weights: [][]float32{
			{0.4, 0.7, -0.2, 0.6, -0.4, 0.3},
			{-0.3, 0.5, 0.1},
		},
	}
	api.NPS.Save(id, &params)
	defer api.NPS.Delete(id)

	// Get a test server
	ts := httptest.NewServer(api.Router)

	// Evaluate a point
	inputs := map[string]float32{"subs": -1, "events": 1}
	raw, _ := json.Marshal(inputs)
	resp, err := http.Post(ts.URL+base+"/v1/nets/"+id+"/evaluate", "application/json", bytes.NewBuffer(raw))
	if err != nil {
		t.Fatalf("A valid POST to the evaluate endpoint returned an error (%s)", err.Error())
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("A valid POST to the evaluate endpoint returned an unexpected status code (%s)", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body (%s)", err.Error())
	}
	var outputs map[string]float32
	err = json.Unmarshal(body, &outputs)
	if err != nil {
		t.Fatalf("Failed to parse response body (%s)", err.Error())
	}
	if len(outputs) == 1 && outputs["size"] != -0.1806419 {
		t.Errorf("Output is incorrect, expected %f got %f", -0.1806419, outputs["size"])
	}
}
