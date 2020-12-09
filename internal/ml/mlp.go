package ml

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/ml/paramstores"
	"github.com/qvantel/nerd/internal/series/pointstores"
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
		Accuracy:   np.Accuracy,
		Averages:   np.Averages,
		Deviations: np.Deviations,
		ErrMargin:  np.ErrMargin,
		Inputs:     np.Inputs,
		Outputs:    np.Outputs,
		Type:       "mlp",
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

// MLP holds the neurons and thus serves to keep track of the values through the network during training and
// operation
type MLP struct {
	ID         string `json:"-"`
	Params     MLPParams
	ParamStore paramstores.NetParamStore `json:"-"`
	neurons    [][]Neuron
	nCount     int
}

// MLPTopology returns an array of neurons per layer given the number of inputs, outputs and hidden layers
func MLPTopology(inputs, outputs, hLayers int) []int {
	topology := make([]int, hLayers+2)
	for i := 0; i <= hLayers; i++ { // <= so that we also fill in the number of neurons for the first layer
		topology[i] = inputs
	}
	topology[hLayers+1] = outputs
	return topology
}

// NewMLP returns a multilayer perceptron net built from scratch with the requested inputs, outputs and hidden layers
func NewMLP(id string, inputs, outputs []string, hLayers int, mls paramstores.NetParamStore, conf config.Config) (*MLP, error) {
	params := MLPParams{
		Accuracy:       -1,
		ActivationFunc: conf.ML.ActivationFunc,
		Epoch:          0,
		ErrMargin:      0,
		Inputs:         inputs,
		LearningRate:   conf.ML.Alpha,
		Topology:       MLPTopology(len(inputs), len(outputs), hLayers),
		Outputs:        outputs,
		Weights:        nil,
	}
	params.Weights = generateWeights(params.Topology)
	err := mls.Save(id, &params)
	if err != nil {
		logger.Error("Error attempting to save params to store", err)
		return nil, err
	}

	return MLPFromParams(id, params, mls)
}

// MLPFromParams returns a multilayer perceptron network initialized with the specified params
func MLPFromParams(id string, np MLPParams, mls paramstores.NetParamStore) (*MLP, error) {
	net := MLP{ID: id, Params: np, ParamStore: mls}
	last := len(net.Params.Topology) - 1
	net.neurons = make([][]Neuron, last+1)
	net.nCount = 0
	for i := range net.Params.Topology {
		size := net.Params.Topology[i]
		if i != last {
			size++
		}
		net.nCount += size
		net.neurons[i] = make([]Neuron, size)
		for j := 0; j < size; j++ {
			net.neurons[i][j] = Neuron{Delta: 0.0, Value: 1}
			if i == 0 || (i != last && j == 0) { // The first layer and the bias neuron for each hidden layer don't have inputs
				continue
			}
			section := len(net.neurons[i-1]) * j
			if i != last {
				section -= len(net.neurons[i-1]) // Adjustment for layers with a bias neuron at j=0
			}
			for n := range net.neurons[i-1] {
				logger.Trace(fmt.Sprintf("[MLP %s] Connecting L%dN%d to L%dN%d with weight %d:%d", net.ID, i-1, n, i, j, i-1, n+section))
				net.neurons[i][j].AddInput(&net.neurons[i-1][n], &net.Params.Weights[i-1][n+section])
			}
		}
	}

	return &net, nil
}

// rmse calculates the root mean square of error
func rmse(pairs, neurons int, diffc float32) float32 {
	return diffc / float32(pairs*neurons)
}

func (net *MLP) normalize(label string, value float32) float32 {
	avg, ok := net.Params.Averages[label]
	if !ok {
		return value
	}
	dev, ok := net.Params.Deviations[label]
	if !ok {
		return value
	}
	return (value - avg) / dev
}

func (net *MLP) denormalize(label string, nValue float32) float32 {
	avg, ok := net.Params.Averages[label]
	if !ok {
		return nValue
	}
	dev, ok := net.Params.Deviations[label]
	if !ok {
		return nValue
	}
	return nValue*dev + avg
}

func (net *MLP) updateNormParams(points []pointstores.Point, tStart, tEnd int) error {
	// TODO: There should be a check somewhere to ensure these aren't updated when the new data set is smaller
	nTrain := float32(len(points))
	if tStart >= 0 {
		nTrain -= float32(tEnd - tStart)
	}
	if nTrain <= 1 {
		logger.Warning("[MLP " + net.ID + "] There are not enough patterns to update the net's normalization parameters")
		return nil // Not strictly an error because this alone would mean no normalization at worst
	}
	net.Params.Averages = map[string]float32{}
	net.Params.Deviations = map[string]float32{}
	// Calculate averages
	for label := range points[0].Values {
		net.Params.Averages[label] = 0
		for i := 0; i < len(points); i++ {
			// Skip the points earmarked for testing
			if i == tStart {
				i = tEnd - 1 // -1 because i will get a +1 before the next iteration
				continue
			}
			net.Params.Averages[label] += points[i].Values[label]
		}
	}
	for label := range net.Params.Averages {
		net.Params.Averages[label] /= nTrain
	}
	// Calculate standard deviations
	for label := range points[0].Values {
		net.Params.Deviations[label] = 0
		for i := 0; i < len(points); i++ {
			// Skip the points earmarked for testing
			if i == tStart {
				i = tEnd - 1 // -1 because i will get a +1 before the next iteration
				continue
			}
			net.Params.Deviations[label] += (points[i].Values[label] - net.Params.Averages[label]) * (points[i].Values[label] - net.Params.Averages[label])
		}
	}
	for label := range net.Params.Deviations {
		net.Params.Deviations[label] = float32(math.Sqrt(float64(net.Params.Deviations[label] / (nTrain - 1))))
		if net.Params.Deviations[label] == 0 {
			return errors.New("the param " + label + " never changes in the training set, normalization won't work")
		}
	}
	return nil
}

// Train will use the specified input/output pairs to modify the net so the behaviour of its connections is closer to
// that of the unknown relationships it's intended to mimic
func (net *MLP) Train(points []pointstores.Point, maxEpoch int, errMargin, testSet, tolerance float32) (float32, error) {
	var diffc float32
	rmseOld := float32(1.0)
	rmseNew := float32(-1.0)
	// Determine test set range
	tStart := -1
	tEnd := -1
	nPoints := len(points)
	nTest := int(math.Floor(float64(float32(nPoints) * testSet)))
	logger.Debug(fmt.Sprintf("Training %s with %d patterns, %d of which will be used for testing", net.ID, nPoints, nTest))
	if testSet > 0 {
		rand.Seed(time.Now().UnixNano())
		max := nPoints - nTest
		tStart = rand.Intn(max + 1)
		tEnd = tStart + nTest
	}
	// Update normalization params (note that the values in points won't be touched)
	err := net.updateNormParams(points, tStart, tEnd)
	if err != nil {
		return 0, err
	}

	for net.Params.Epoch = 0; net.Params.Epoch < maxEpoch && float32(math.Abs(1-float64(rmseNew/rmseOld))) >= tolerance; net.Params.Epoch++ {
		for i := 0; i < nPoints; i++ {
			// Skip the points earmarked for testing
			if i == tStart {
				i = tEnd - 1 // -1 because i will get a +1 before the next iteration
				continue
			}
			outputs, err := net.Evaluate(points[i].Values)
			if err != nil {
				return 0, err
			}
			err = net.backpropagate(points[i].Values)
			if err != nil {
				return 0, err
			}
			// Calculate error (note that outputs contains denormalized values, same as points)
			diffc = float32(0.0)
			for label := range outputs {
				diffc += (outputs[label] - points[i].Values[label]) * (outputs[label] - points[i].Values[label])
			}
		}
		rmseOld = rmseNew
		rmseNew = rmse(nPoints, net.nCount, diffc)
		err := net.ParamStore.Save(net.ID, &net.Params)
		if err != nil {
			return 0, err
		}
	}
	net.Params.Epoch = 0
	if testSet <= 0 {
		err := net.ParamStore.Save(net.ID, &net.Params)
		return -1.0, err
	}
	errs := 0
	for i := tStart; i < tEnd; i++ {
		outputs, err := net.Evaluate(points[i].Values)
		if err != nil {
			return 0, err
		}
		for _, label := range net.Params.Outputs {
			// Note that outputs contains denormalized values, same as points
			if math.Abs(float64(outputs[label]-points[i].Values[label])) > float64(errMargin) {
				errs++
				break
			}
		}
	}
	net.Params.ErrMargin = errMargin
	net.Params.Accuracy = 1.0 - float32(errs)/float32(nTest)
	err = net.ParamStore.Save(net.ID, &net.Params)
	if err != nil {
		return 0, err
	}
	return net.Params.Accuracy, nil
}

func (net *MLP) addWeight(iL, iN, oN int, weight float32) {
	section := len(net.neurons[iL]) * oN
	if iL+1 != len(net.neurons)-1 {
		section -= len(net.neurons[iL]) // Adjustment for layers with a bias neuron at j=0
	}
	net.Params.Weights[iL][iN+section] += weight
}

func (net *MLP) backpropagate(target map[string]float32) error {
	last := len(net.neurons) - 1
	// Calculate the error for the last layer (normalizing the target outputs to match the normalized neuron values)
	for i, label := range net.Params.Outputs {
		net.neurons[last][i].Delta = (net.normalize(label, target[label]) - net.neurons[last][i].Value) * DerivedF(net.neurons[last][i].Value)
	}
	// Propagate to the hidden layers
	for layer := last - 1; layer > 0; layer-- {
		for n := range net.neurons[layer] {
			net.neurons[layer][n].RefreshDelta()
		}
	}
	// Update weights
	for layer := last; layer > 0; layer-- {
		for i, out := range net.neurons[layer] {
			for j, in := range out.inputs {
				//logger.Trace(fmt.Sprintf("dL%dN%dO%d = %f * %f * %f", layer, i, j, net.Params.LearningRate, out.Delta, in.n.Value))
				net.addWeight(layer-1, j, i, net.Params.LearningRate*out.Delta*in.n.Value)
			}
		}
	}
	return nil
}

// Evaluate will return the net's output for the given input vector
func (net *MLP) Evaluate(inputs map[string]float32) (map[string]float32, error) {
	if len(inputs) < len(net.neurons[0])-1 {
		return nil, fmt.Errorf(
			"Number of inputs must match the number of neurons in the first layer, expected %d got %d",
			len(net.neurons[0])-1,
			len(inputs),
		)
	}
	// Load the values into the first layer
	for n, label := range net.Params.Inputs {
		net.neurons[0][n+1].Value = net.normalize(label, inputs[label]) // +1 to avoid the bias neuron
	}

	// Propagate them
	for layer := 1; layer < len(net.neurons); layer++ {
		for n := range net.neurons[layer] {
			net.neurons[layer][n].RefreshValue()
			//logger.Trace(fmt.Sprintf("L%dN%d = %f", layer, n, net.neurons[layer][n].Value))
		}
	}

	outputs := map[string]float32{}
	for n, label := range net.Params.Outputs {
		outputs[label] = net.denormalize(label, net.neurons[len(net.neurons)-1][n].Value)
	}

	return outputs, nil
}

func generateWeights(neurons []int) [][]float32 {
	weights := make([][]float32, len(neurons)-1)

	for i := range neurons[:len(neurons)-1] {
		size := (neurons[i] + 1) * neurons[i+1]
		weights[i] = make([]float32, size)
		for j := range weights[i] {
			rand.Seed(time.Now().UnixNano())
			weights[i][j] = rand.Float32() - 0.5
		}
	}

	return weights
}
