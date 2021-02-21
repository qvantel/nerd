package nets

import "testing"

func TestAddInput(t *testing.T) {
	a := Neuron{Value: 1}
	b := Neuron{Value: 2}

	var w float32 = 0.5
	b.AddInput(&a, &w)

	if b.inputs[0].n.outputs[0].n != &b {
		t.Errorf("Failed to find the expected bidirectional connection")
	}
}

func TestRefreshValue(t *testing.T) {
	a := Neuron{Value: 1}

	got := a.RefreshValue()
	if got != 1 {
		t.Errorf("RefreshValue shouldn't have any effect on first layer or bias neurons, expected 1 got %f", got)
	}
	if got != a.Value {
		t.Errorf("RefreshValue should return the neuron's new value, expected %f got %f", a.Value, got)
	}
}
