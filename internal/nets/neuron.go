package nets

import (
	"math"
)

type synapse struct {
	n      *Neuron
	weight *float32
}

// Neuron holds the state of the smallest component in a neural net
type Neuron struct {
	Delta   float32
	inputs  []synapse
	outputs []synapse
	Value   float32
}

// AddInput configures a connection between two neurons
func (n *Neuron) AddInput(a *Neuron, weight *float32) {
	a.outputs = append(a.outputs, synapse{n, weight})
	n.inputs = append(n.inputs, synapse{a, weight})
}

// RefreshValue will update a neuron's output with the current value and weight of its inputs
func (n *Neuron) RefreshValue() float32 {
	// If the neuron doesn't have any inputs, it's either in the first layer or a bias neuron, so we shouldn't touch it
	if len(n.inputs) == 0 {
		return n.Value
	}
	var yIn float32 = 0.0
	for _, input := range n.inputs {
		yIn += input.n.Value * *input.weight
	}
	n.Value = F(yIn)
	return n.Value
}

// RefreshDelta will update a neuron's deviation from the desired output
func (n *Neuron) RefreshDelta() float32 {
	// If the neuron doesn't have any outputs, it's in the last layer, so we shouldn't touch it
	if len(n.outputs) == 0 {
		return n.Delta
	}
	var dIn float32 = 0.0
	for _, output := range n.outputs {
		dIn += output.n.Delta * *output.weight
	}
	n.Delta = dIn * DerivedF(n.Value)
	return n.Delta
}

// F is the bipolar sigmoid function
func F(x float32) float32 {
	return float32((2.0 / (1 + math.Pow(math.E, float64(-x)))) - 1)
}

// DerivedF is the derivative of the bipolar sigmoid
func DerivedF(y float32) float32 {
	return 0.5 * (1 + y) * (1 - y)
}
