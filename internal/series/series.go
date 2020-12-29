// Package series contains the logic that manages the datasets used for machine learning
package series

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/ml"
	"github.com/qvantel/nerd/internal/series/pointstores"
)

// ProcessUpdate serves to separate the cloud event processing logic from that which is Kafka specific, that way
// allowing for training data to be ingested into the system through other channels
func ProcessUpdate(event event.Event, ps pointstores.PointStore, tServ chan types.TrainRequest, conf config.Config) error {
	switch event.Type() {
	case "com.qvantel.nerd.metricsupdate":
		// Unmarshal
		var mu types.MetricsUpdate
		err := json.Unmarshal(event.Data(), &mu)
		if err != nil {
			return err
		}
		if mu.Stage != "test" && mu.Stage != "production" {
			return errors.New("the stage of a metrics update must be test or production, got " + mu.Stage + " instead")
		}
		inputs := []string{}
		for name := range mu.Points[0].Inputs {
			inputs = append(inputs, name)
		}
		sort.Strings(inputs)
		outputs := []string{}
		for name := range mu.Points[0].Outputs {
			outputs = append(outputs, name)
		}
		sort.Strings(outputs)
		// Get current count
		count, err := ps.GetCount(mu.SeriesID, nil)
		if err != nil {
			return err
		}

		// Persist
		if mu.Labels == nil {
			mu.Labels = map[string]string{}
		}
		mu.Labels["subject"] = event.Subject()
		mu.Labels["stage"] = mu.Stage
		for _, point := range mu.Points {
			values := map[string]float32{}
			for key, value := range point.Inputs {
				values[key] = value
			}
			for key, value := range point.Outputs {
				values[key] = value
			}
			p := pointstores.Point{
				Labels:    mu.Labels,
				Values:    values,
				TimeStamp: point.TimeStamp,
			}
			err = ps.AddPoint(mu.SeriesID, p)
			if err != nil {
				logger.Error("Error encountered while persisting point to store", err)
				return err
			}
		}
		// Queue up training if enough points are available
		req, _ := ml.Required(len(inputs), 1, conf.ML.HLayers, conf) // 1 because we'll be creating individual nets for each output
		logger.Trace(fmt.Sprintf("Got %d points for %s, %d required for training", count+len(mu.Points), mu.SeriesID, req))
		if count < req && count+len(mu.Points) >= req {
			if conf.Series.StoreType == "elasticsearch" {
				// Pause for refresh, otherwise the points might not be readable yet and/or the next call might see the
				// old count again
				time.Sleep(1 * time.Second)
			}
			sort.Strings(inputs)
			sort.Strings(outputs)
			tServ <- types.TrainRequest{
				ErrMargin: mu.ErrMargin,
				Inputs:    inputs,
				Outputs:   outputs,
				Required:  req,
				SeriesID:  mu.SeriesID,
			}
		}

		return nil
	default:
		logger.Warning("Received cloud event with unsupported type (" + event.Type() + ") from " + event.Source())
		return errors.New("unsupported event type")
	}
}
