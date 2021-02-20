// Package nets provides a standard interface for interacting with neural networks
package nets

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"math"
	"strings"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/nets/paramstores"
	"github.com/qvantel/nerd/internal/series/pointstores"
)

// Network represents the neural net implementation that is being used
type Network interface {
	// This method should return the outputs of feeding the specified inputs into the net
	Evaluate(inputs map[string]float32) (map[string]float32, error)
	ID() string
	// This method should return the net's params
	Params() paramstores.NetParams
	// This method should feed the training pairs into the net and update its params in the net param storage when
	// it's done
	Train(points []pointstores.Point, maxEpoch int, errMargin, testSet, tolerance float32) (float32, error)
}

// NewNetwork returns an initialized neural network of the type specified in the configuration
func NewNetwork(id string, inputs, outputs []string, chromosome Chromosome) (Network, error) {
	switch chromosome.Type {
	case types.MultilayerPerceptron:
		return NewMLP(id, inputs, outputs, chromosome)
	default:
		return nil, errors.New(chromosome.Type + " is not a valid net type")
	}
}

// List encapsulates the logic required to fill in BriefNet objects from the IDs of nets in the store
func List(offset, limit int, pattern string, nps paramstores.NetParamStore) ([]types.BriefNet, int, error) {
	nets := []types.BriefNet{}
	ids, cursor, err := nps.List(offset, limit, pattern)
	if err != nil {
		return nil, 0, err
	}
	for _, id := range ids {
		nType, err := ID2Type(id)
		if err != nil {
			logger.Warning("Encountered incorrectly formatted key in Redis (" + id + ")")
			continue
		}
		switch nType {
		case types.MultilayerPerceptron:
			var np paramstores.MLPParams
			found, err := nps.Load(id, &np)
			if err != nil {
				return nil, 0, err
			}
			if found {
				brief := np.Brief()
				brief.ID = id
				nets = append(nets, *brief)
			}
		default:
			logger.Warning("Encountered incorrectly formatted key in Redis (" + id + ")")
			continue
		}
	}
	return nets, cursor, nil
}

// LoadNetwork is used to retrieve network params for a given ID and build a network object with them. When they can't
// be found (nil, nil) will be returned
func LoadNetwork(id, nType string, nps paramstores.NetParamStore) (Network, error) {
	switch nType {
	case types.MultilayerPerceptron:
		var np paramstores.MLPParams
		found, err := nps.Load(id, &np)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		return MLPFromParams(id, np)
	default:
		return nil, errors.New(nType + " is not a valid net type")
	}
}

// Required returns the number of patterns required to train a net (of the most demanding type) with the specified
// topology
func Required(inputs, outputs, hLayers int, conf config.Config) int {
	// required := []int{}

	// MLP
	topology := MLPTopology(inputs, outputs, hLayers)
	w := 0
	for layer, neurons := range topology[:len(topology)-1] {
		w += (neurons + 1) * topology[layer+1]
	}
	/*
		required = append(required, int(math.Ceil(float64(w)/0.1)))

		max := 0
		for index := range required {
			if required[index] > max {
				max = required[index]
			}
		}
		return max
	*/

	return int(math.Ceil(float64(w) / 0.1))
}

// Trainer listens for requests to train neural nets with new points
func Trainer(c chan types.TrainRequest, conf config.Config) error {
	// Set up net param store
	nps, err := paramstores.New(conf)
	if err != nil {
		logger.Error("Failed to initialize net param store for the training service", err)
		return err
	}
	// Set up point store
	ps, err := pointstores.New(conf)
	if err != nil {
		logger.Error("Failed to initialize point store for the training service", err)
		return err
	}
	logger.Info("Training service initialized")
	for tr := range c {
		group := tr.SeriesID + "-" + hash(tr.Inputs)
		logger.Info("Training group " + group)
		// Get points
		points, err := ps.GetLastN(tr.SeriesID, nil, tr.Required)
		if err != nil {
			logger.Error("Error retrieving points from store for series "+tr.SeriesID, err)
			return err
		}
		// Build and train nets (1 per output)
		for index := range tr.Outputs {
			pop := NewPopulation(conf.ML)
			net, err := pop.Optimal(tr, tr.Outputs[index:index+1], points)
			if err != nil {
				logger.Error("Error training net from "+tr.SeriesID+" for "+tr.Outputs[index], err)
				continue // We can't kill the whole service every time training fails
			}
			err = nps.Save(net.ID(), net.Params())
			if err != nil {
				logger.Error("Error saving net", err)
				return err
			}
		}
		logger.Info("Training for group " + group + " completed")
	}
	return nil
}

// ID2Type takes a string and attempts to extract a net ID from it, when it comes to errors there can be false negatives
// but not false positives (no error doesn't necessarily mean it's a good ID)
func ID2Type(id string) (string, error) {
	parts := strings.Split(id, "-")
	if len(parts) < 4 {
		return "", errors.New("incorrectly formatted net ID")
	}
	return parts[len(parts)-1], nil
}

// hash takes a list of SORTED strings and returns its hash
func hash(keys []string) string {
	hash := sha1.New()
	for _, key := range keys {
		hash.Write([]byte(key))
	}
	raw := hash.Sum(nil)
	res := make([]byte, hex.EncodedLen(len(raw)))
	hex.Encode(res, raw)

	return string(res)
}
