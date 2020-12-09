// Package ml provides a standard interface for interacting with neural networks
package ml

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"math"
	"strings"

	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/ml/paramstores"
	"github.com/qvantel/nerd/internal/series/pointstores"
)

// Network represents the neural net implementation that is being used
type Network interface {
	// This method should feed the training pairs into the net and update its params in the net param storage when
	// it's done
	Train(points []pointstores.Point, maxEpoch int, errMargin, testSet, tolerance float32) (float32, error)
	// This method should return the outputs of feeding the specified inputs into the net
	Evaluate(inputs map[string]float32) (map[string]float32, error)
}

// NewNetwork returns an initialized neural network of the type specified in the configuration
func NewNetwork(id string, inputs, outputs []string, hLayers int, create bool, mls paramstores.NetParamStore, conf config.Config) (Network, error) {
	nType, err := ID2Type(id)
	if err != nil {
		return nil, err
	}
	switch nType {
	case "mlp":
		var np MLPParams
		found, err := mls.Load(id, &np)
		if err != nil {
			logger.Error("Error attempting to load params for net "+id+" from store", err)
			return nil, err
		}
		if found {
			logger.Debug("Net params for net " + id + " found")
			return MLPFromParams(id, np, mls)
		}
		if create {
			logger.Debug("Net params for net " + id + " not found, creating from scratch")
			return NewMLP(id, inputs, outputs, hLayers, mls, conf)
		}
	default:
		return nil, errors.New(nType + " is not a valid net type")
	}
	// Valid type but not found and create is false
	logger.Debug("Net params for net " + id + " not found")
	return nil, nil
}

// List encapsulates the logic required to fill in BriefNet objects from the IDs of nets in the store
func List(offset, limit int, mls paramstores.NetParamStore) ([]types.BriefNet, int, error) {
	var nets []types.BriefNet
	ids, cursor, err := mls.List(offset, limit)
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
		case "mlp":
			var np MLPParams
			found, err := mls.Load(id, &np)
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

// Required returns the number of patterns required to train a net with the specified topology
func Required(inputs, outputs, hLayers int, conf config.Config) (int, error) {
	switch conf.ML.Net {
	case "mlp":
		topology := MLPTopology(inputs, outputs, hLayers)
		w := 0
		for layer, neurons := range topology[:len(topology)-1] {
			w += (neurons + 1) * topology[layer+1]
		}
		return int(math.Ceil(float64(w) / 0.1)), nil
	default:
		return 0, errors.New(conf.ML.Net + " is not a valid net type")
	}
}

// Trainer listens for requests to train neural nets with new points
func Trainer(c chan types.TrainRequest, conf config.Config) error {
	// Set up ml store
	mls, err := paramstores.New(conf)
	if err != nil {
		logger.Error("Failed to initialize ml store for the training service", err)
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
		for _, output := range tr.Outputs {
			id := group + "-" + hash([]string{output}) + "-" + conf.ML.Net
			net, err := NewNetwork(id, tr.Inputs, []string{output}, conf.ML.HLayers, true, mls, conf)
			if err != nil {
				logger.Error("Error creating net from "+tr.SeriesID+" for "+output, err)
				return err
			}
			_, err = net.Train(points, conf.ML.MaxEpoch, tr.ErrMargin, conf.ML.TestSet, conf.ML.Tolerance)
			if err != nil {
				logger.Error("Error training net from "+tr.SeriesID+" for "+output, err)
				continue // We can't kill the whole service every time training fails
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
		return "", errors.New("Incorrectly formatted net ID")
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
