package paramstores

import "github.com/qvantel/nerd/api/types"

func initTest(nps NetParamStore) (string, error) {
	id := "test-evaluate-51e1890284194a8e4bb9923994e46cf59cfdd90d-89368e1d68015693ab48ee189d0632cb5d6edfb3-" + types.MultilayerPerceptron
	params := MLPParams{
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
	return id, nps.Save(id, &params)
}
