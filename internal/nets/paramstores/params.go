package paramstores

import (
	"encoding/json"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/logger"
)

// MLPParams holds the minimum information required to rebuild the net from scratch plus some metadata that is required
// for other parts of the system
type MLPParams struct {
	Accuracy       float32
	ActivationFunc string
	Averages       map[string]float32
	Deviations     map[string]float32
	Epoch          int
	ErrMargin      float32
	Inputs         []string
	LearningRate   float32
	Topology       []int
	Outputs        []string
	Weights        [][]float32
}

// Brief returns a standard summarized version of the net's params (not enough to rebuild it but enough to compare it)
func (np MLPParams) Brief() *types.BriefNet {
	return &types.BriefNet{
		Accuracy:       np.Accuracy,
		ActivationFunc: np.ActivationFunc,
		Averages:       np.Averages,
		Deviations:     np.Deviations,
		ErrMargin:      np.ErrMargin,
		HLayers:        len(np.Topology) - 2,
		Inputs:         np.Inputs,
		LearningRate:   np.LearningRate,
		Outputs:        np.Outputs,
		Type:           types.MultilayerPerceptron,
	}
}

// Unmarshal is used to tell the param store how to read a NetParams object for an MLP net
func (np *MLPParams) Unmarshal(b []byte) error {
	return json.Unmarshal(b, np)
}

// Marshal is used to tell the param store how to write a NetParams object for an MLP net
func (np *MLPParams) Marshal() ([]byte, error) {
	return json.Marshal(np)
}

func (np MLPParams) String() string {
	data, err := np.Marshal()
	if err != nil {
		logger.Error("There was an error marshalling the net params", err)
		return ""
	}
	return string(data)
}
