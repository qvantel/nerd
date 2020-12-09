package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/oklog/run"
	"github.com/qvantel/nerd/api"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/internal/config"
	"github.com/qvantel/nerd/internal/logger"
	"github.com/qvantel/nerd/internal/ml"
	"github.com/qvantel/nerd/internal/series"
	"github.com/segmentio/kafka-go"
)

func main() {
	// Get config
	conf, err := config.New()
	// Initialize logger (even if the previous statement returns an error, the logging part should be filled in)
	logger.Init(*conf)
	logger.Info("Initializing component")
	if err != nil {
		logger.Error("Error encountered while loading configuration", err)
		return
	}

	var g run.Group

	// Initialize training service
	tServ := make(chan types.TrainRequest, 10)
	g.Add(func() error { return ml.Trainer(tServ, *conf) }, func(error) { close(tServ) })

	// Initialize consumer
	consumer := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  conf.Series.Source.Brokers,
		GroupID:  conf.Series.Source.GroupID,
		Topic:    conf.Series.Source.Topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	g.Add(func() error { return series.Consumer(consumer, tServ, *conf) }, func(error) { consumer.Close() })

	// Initialize API
	api, err := api.New(tServ, *conf)
	if err != nil {
		logger.Error("Error encountered initializing API", err)
		return
	}
	srv := &http.Server{
		Addr:    ":5400",
		Handler: api.Router,
	}
	g.Add(func() error {
		return srv.ListenAndServe()
	}, func(error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server forced to shutdown", err)
			os.Exit(1)
		}
	})

	err = g.Run()
	if err != nil {
		logger.Error("Critical error encountered, exiting", err)
	}
}
