package nets

import (
	"encoding/json"
	"testing"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/nets/paramstores"
	"github.com/qvantel/nerd/internal/series/pointstores"
)

func TestEvaluate(t *testing.T) {
	nps := paramstores.FileAdapter{Path: "."}
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
	nps.Save(t.Name(), &params)
	defer nps.Delete(t.Name())

	net, _ := MLPFromParams(t.Name(), params)

	out, err := net.Evaluate(map[string]float32{"subs": -1, "events": 1})
	if err != nil {
		t.Fatalf("Failed to evaluate the test scenario (%s)", err.Error())
	}

	if len(out) == 1 && out["size"] != -0.1806419 {
		t.Errorf("Output is incorrect, expected %f got %f", -0.1806419, out["size"])
	}
}

func TestAddWeight(t *testing.T) {
	nps := paramstores.FileAdapter{Path: "."}
	params := paramstores.MLPParams{
		ActivationFunc: types.BipolarSigmoid,
		Inputs:         []string{"subs", "events"},
		LearningRate:   0.25,
		Topology:       []int{2, 2, 1},
		Outputs:        []string{"size"},
		Weights: [][]float32{
			{0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
			{0.1, 0.1, 0.1},
		},
	}
	nps.Save(t.Name(), &params)
	defer nps.Delete(t.Name())

	net, _ := MLPFromParams(t.Name(), params)

	net.addWeight(0, 0, 1, 0.1)
	if net.params.Weights[0][0] != 0.2 {
		t.Fatalf("Output is incorrect, expected %f got %f", 0.2, net.params.Weights[0][0])
	}

	net.addWeight(1, 1, 0, 0.2)
	if net.params.Weights[1][1] != 0.3 {
		t.Errorf("Output is incorrect, expected %f got %f", 0.3, net.params.Weights[1][1])
	}
}

func TestTrain(t *testing.T) {
	nps := paramstores.FileAdapter{Path: "."}
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
	want := paramstores.MLPParams{
		ActivationFunc: types.BipolarSigmoid,
		Inputs:         []string{"subs", "events"},
		LearningRate:   0.25,
		Topology:       []int{2, 2, 1},
		Outputs:        []string{"size"},
		Weights: [][]float32{
			{0.43355018, 0.6664498, -0.16644982, 0.6048054, -0.40480542, 0.30480543},
			{-0.15723555, 0.4650343, 0.18161416},
		},
	}
	nps.Save(t.Name(), &params)
	defer nps.Delete(t.Name())

	net, _ := MLPFromParams(t.Name(), params)

	_, err := net.Train([]pointstores.Point{{Values: map[string]float32{"subs": -1, "events": 1, "size": 1}}}, 1, net.params.ErrMargin, 0, 0.1)
	if err != nil {
		t.Fatalf("Failed to execute the training test scenario (%s)", err.Error())
	}

	if net.params.String() != want.String() {
		t.Errorf("Training resulted in incorrect values, expected %s got %s", want.String(), net.params.String())
	}
}

func TestTrain2(t *testing.T) {
	var storeParams map[string]interface{}
	json.Unmarshal([]byte(`{"Path": "."}`), &storeParams)
	nps := paramstores.FileAdapter{Path: "."}
	net, _ := NewMLP(
		t.Name(),
		[]string{"value-0", "value-1", "value-2", "value-3", "value-4", "value-5", "value-6", "value-7", "value-8"},
		[]string{"value-9", "value-10"},
		Chromosome{
			ActivationFunc: types.BipolarSigmoid,
			HLayers:        1,
			LearningRate:   0.003,
		},
	)
	defer nps.Delete(t.Name())

	ps := pointstores.FileAdapter{Path: "."}

	points, err := ps.LoadTestSet("../../test/normalization_test_data.txt")
	if err != nil {
		t.Fatalf("Failed to load test data (%s)", err.Error())
	}

	net.Train(points, 1000, 0.49999999, 0.4, 0.1)
	if net.params.Accuracy < 0.9 {
		t.Errorf("Expected at least 90 percent accuracy for this test data, got: %f", net.params.Accuracy)
	}
}
