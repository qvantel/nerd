package series

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/series/pointstores"
	kafka "github.com/segmentio/kafka-go"
)

// Consumer starts a Kafka consumer that'll ingest metrics updates
func Consumer(consumer *kafka.Reader, tServ chan types.TrainRequest, conf config.Config) (e error) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				e = errors.New(x)
			case error:
				e = x
			default:
				e = errors.New("unknown panic")
			}
		}
	}()

	// Set up point store
	ps, err := pointstores.New(conf)
	if err != nil {
		logger.Error("Failed to initialize point store", err)
		return err
	}

	pFailures := 0
	ctx := context.Background()
	logger.Info("Consumer initialized, now reading from " + consumer.Config().Topic)
	for pFailures < conf.Series.FailLimit {
		m, err := consumer.FetchMessage(ctx)
		if err != nil {
			logger.Error("Consumer failed to fetch new message", err)
			return err
		}

		logger.Trace(
			fmt.Sprintf(
				"Message received at topic/partition/offset %v/%v/%v: %s = %s",
				m.Topic,
				m.Partition,
				m.Offset,
				string(m.Key),
				string(m.Value),
			),
		)

		event := cloudevents.NewEvent()
		err = json.Unmarshal(m.Value, &event)
		if err != nil {
			logger.Warning("Consumer failed to unmarshal message (" + err.Error() + ")")
			pFailures++
			continue
		}

		err = ProcessUpdate(event, ps, tServ, conf)
		if err != nil {
			logger.Error("Failed to process message", err)
			pFailures++
			continue
		}
		pFailures = 0

		if err := consumer.CommitMessages(ctx, m); err != nil {
			logger.Error("Consumer failed to commit messages", err)
			return err
		}
	}

	return errors.New("reached consecutive processing failure limit")
}
