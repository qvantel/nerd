package nets

import (
	"testing"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/series/pointstores"
)

func TestCrossover(t *testing.T) {
	a := Chromosome{
		ActivationFunc: "act1",
		HLayers:        1,
		LearningRate:   0.01,
		Type:           "type1",
	}
	b := Chromosome{
		ActivationFunc: "act2",
		HLayers:        2,
		LearningRate:   0.02,
		Type:           "type2",
	}
	c, d := a, b
	c.LearningRate, d.LearningRate = b.LearningRate, a.LearningRate
	c.HLayers, d.HLayers = b.HLayers, a.HLayers

	res := a.Crossover(b, 1)
	if len(res) != 2 {
		t.Fatalf("Expected 2 chromosomes from crossover, got %d instead", len(res))
	}
	if res[0] != c || res[1] != d {
		t.Error("Crossover returned incorrect results")
	}
}

func TestMutate(t *testing.T) {
	a := Chromosome{
		ActivationFunc: "act1",
		HLayers:        1,
		LearningRate:   0.01,
		Type:           "type1",
	}
	b := a

	a.Mutate(0)
	if a.ActivationFunc == b.ActivationFunc {
		t.Error("Mutating gene 0 didn't have any effect")
	}

	a.Mutate(1)
	if a.HLayers == b.HLayers {
		t.Error("Mutating gene 1 didn't have any effect")
	}
	if a.HLayers != 2 {
		t.Errorf("When HLayers is 1, the only possible mutation is 2, got %d instead", a.HLayers)
	}
	a.Mutate(1)
	if a.HLayers != 1 && a.HLayers != 3 {
		t.Errorf("When HLayers is 2, the only possible mutations are 1 or 3, got %d instead", a.HLayers)
	}

	a.Mutate(2)
	if a.LearningRate == b.LearningRate {
		t.Error("Mutating gene 2 didn't have any effect")
	}
	if a.LearningRate != 0.02 {
		t.Errorf("When LearningRate is 0.01 the only possible mutation is 0.02, got %f instead", a.LearningRate)
	}
	a.Mutate(2)
	if a.LearningRate != 0.01 && a.LearningRate != 0.03 {
		t.Errorf("When HLayers is 0.02, the only possible mutations are 0.01 or 0.03, got %d instead", a.HLayers)
	}

}

func TestOptimal(t *testing.T) {
	mlConf := config.MLParams{
		Generations: 5,
		MaxEpoch:    1000,
		MaxHLayers:  5,
		MinHLayers:  1,
		StoreType:   config.FileParamStore,
		StoreParams: map[string]interface{}{"Path": "."},
		TestSet:     0.4,
		Tolerance:   0.1,
		Variations:  6,
	}
	pop := NewPopulation(mlConf)
	if len(pop.individuals) != mlConf.Variations {
		t.Fatalf("Expected the same number of individuals as requested variations, got %d", len(pop.individuals))
	}

	ps := pointstores.FileAdapter{Path: "."}
	points, err := ps.LoadTestSet("../../test/normalization_test_data.txt")
	if err != nil {
		t.Fatalf("Failed to load test data (%s)", err.Error())
	}
	tr := types.TrainRequest{
		ErrMargin: 0.49999999,
		Inputs:    []string{"value-0", "value-1", "value-2", "value-3", "value-4", "value-5", "value-6", "value-7", "value-8"},
		Outputs:   []string{"value-9"},
		SeriesID:  "file-test-set",
	}
	net, err := pop.Optimal(tr, tr.Outputs, points)
	if err != nil {
		t.Fatalf("Optimal search failed (%s)", err.Error())
	}

	netAcc := net.Params().Brief().Accuracy
	var highestAccuracy float32 = -1
	for _, individual := range pop.individuals {
		if individual.Fitness > highestAccuracy {
			highestAccuracy = individual.Fitness
		}
	}

	if netAcc < highestAccuracy {
		t.Errorf("Expected optimal net to have the highest accuracy (%f), got %f instead", highestAccuracy, netAcc)
	}
}
