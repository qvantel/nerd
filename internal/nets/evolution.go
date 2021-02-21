package nets

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/series/pointstores"
)

// Chromosome represents a neural network configuration
type Chromosome struct {
	ActivationFunc string
	Fitness        float32 // Aptitude for infering outputs for the given inputs
	HLayers        int     // Number of hidden layers
	LearningRate   float32
	Type           string
	Net            Network
}

// Check trains a network with the chromosome's config and updates its fitness based on the accuracy
func (c *Chromosome) Check(tr types.TrainRequest, outputs []string, points []pointstores.Point, params config.MLParams) error {
	if c.Net != nil {
		return nil
	}
	id := tr.SeriesID + "-" + hash(tr.Inputs) + "-" + hash(outputs) + "-" + c.Type
	var err error
	c.Net, err = NewNetwork(id, tr.Inputs, outputs, *c)
	if err != nil {
		return err
	}
	c.Fitness, err = c.Net.Train(points, params.MaxEpoch, tr.ErrMargin, params.TestSet, params.Tolerance)
	if err != nil {
		return err
	}
	return nil
}

// Crossover exchanges n+1 parameters between the chromosomes to create two new configurations
func (c Chromosome) Crossover(b Chromosome, n int) []Chromosome {
	// Clear the net pointers from the copies of c and b
	c.Net, b.Net = nil, nil

	c.LearningRate, b.LearningRate = b.LearningRate, c.LearningRate
	if n >= 1 {
		c.HLayers, b.HLayers = b.HLayers, c.HLayers
	}
	if n >= 2 {
		c.ActivationFunc, b.ActivationFunc = b.ActivationFunc, c.ActivationFunc
	}
	return []Chromosome{c, b}
}

// decimals returns the number of decimal places of a given float
func decimals(number float32) int {
	count := 0
	for i := float32(10); number < 1; i *= 10 {
		number *= i
		count++
	}
	return count
}

// Mutate randomly alters the given gene
func (c *Chromosome) Mutate(gene int) {
	rand.Seed(time.Now().UnixNano())
	switch gene {
	case 0:
		c.ActivationFunc = randomString(types.ActivationFuncs())
	case 1:
		if c.HLayers == 1 {
			c.HLayers = 2
		} else {
			c.HLayers += rand.Intn(2)*2 - 1
		}
	case 2:
		d := float32(math.Pow10(decimals(c.LearningRate)))
		if c.LearningRate == 1/d {
			c.LearningRate = 2 / d
		} else {
			c.LearningRate += float32(rand.Intn(2)*2-1) / d
		}
	default:
		return
	}
	// Reset the net as it no longer matches the configuration
	c.Net = nil
}

// Population represents a collection of individuals and their metadata
type Population struct {
	first       int
	individuals []Chromosome
	last        int
	params      config.MLParams
	second      int
}

// NewPopulation creates a new set of individuals and initializes their metadata
func NewPopulation(params config.MLParams) *Population {
	pop := Population{
		first:  -1,
		last:   -1,
		params: params,
		second: -1,
	}
	pop.individuals = make([]Chromosome, params.Variations)
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < params.Variations; i++ {
		pop.individuals[i] = Chromosome{
			ActivationFunc: randomString(types.ActivationFuncs()),
			Fitness:        -1,
			HLayers:        rand.Intn(params.MaxHLayers+1) + params.MinHLayers,
			LearningRate:   float32(rand.Intn(999)+1) / 1000,
			Type:           randomString(types.Nets()),
		}
	}
	return &pop
}

// Optimal uses a genetic algorithm to find the config that results in the most accurate net for the given points and
// returns that net
func (pop *Population) Optimal(tr types.TrainRequest, outputs []string, points []pointstores.Point) (Network, error) {
	for gen := 0; gen < pop.params.Generations; gen++ {
		rand.Seed(time.Now().UnixNano())
		// Calculate fitness for each individual
		err := pop.rank(tr, outputs, points)
		if err != nil {
			return nil, err
		}
		logger.Debug(fmt.Sprintf(
			"Gen %d fitness: first = %f, second = %f, last = %f",
			gen,
			pop.individuals[pop.first].Fitness,
			pop.individuals[pop.second].Fitness,
			pop.individuals[pop.last].Fitness,
		))

		// Cross fittest individuals
		cp := rand.Intn(3)
		offspring := pop.individuals[pop.first].Crossover(pop.individuals[pop.second], cp)

		// Mutate offspring (20% chance)
		mc := rand.Intn(5)
		if mc == 0 {
			gene := rand.Intn(3)
			offspring[0].Mutate(gene)
			gene = rand.Intn(3)
			offspring[1].Mutate(gene)
		}

		// Calculate offspring fitness
		for index := range offspring {
			err := offspring[index].Check(tr, outputs, points, pop.params)
			if err != nil {
				return nil, err
			}
		}

		// Replace least fit individual with fittest offspring
		if offspring[0].Fitness > offspring[1].Fitness {
			pop.individuals[pop.last] = offspring[0]
		} else {
			pop.individuals[pop.last] = offspring[1]
		}
	}

	// If the fittest offspring is better, return it instead
	if pop.individuals[pop.last].Fitness > pop.individuals[pop.first].Fitness {
		return pop.individuals[pop.last].Net, nil
	}

	return pop.individuals[pop.first].Net, nil
}

// randomString returns a randomly selected string from a slice
func randomString(options []string) string {
	i := rand.Intn(len(options))
	return options[i]
}

// rank traverses the population calculating the fitness of each individual and identifying the fittest, second fittest
// and least fit
func (pop *Population) rank(tr types.TrainRequest, outputs []string, points []pointstores.Point) error {
	pop.first, pop.second, pop.last = -1, -1, -1
	for index := range pop.individuals {
		err := pop.individuals[index].Check(tr, outputs, points, pop.params)
		if err != nil {
			return err
		}
		if pop.first == -1 || pop.individuals[index].Fitness > pop.individuals[pop.first].Fitness {
			if pop.last == -1 {
				pop.last = pop.second
			}
			pop.second, pop.first = pop.first, index
		} else if pop.second == -1 || pop.individuals[index].Fitness > pop.individuals[pop.second].Fitness {
			if pop.last == -1 {
				pop.last = pop.second
			}
			pop.second = index
		} else if pop.last == -1 || pop.individuals[index].Fitness < pop.individuals[pop.last].Fitness {
			pop.last = index
		}
	}
	return nil
}
