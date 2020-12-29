package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/qvantel/nerd/api/types"
	"github.com/qvantel/nerd/pkg/producer"
)

func main() {
	// Get arguments
	var (
		batchSize, inN                int
		errMargin                     float64
		headers                       bool
		pc                            producer.Config
		sep, seriesID, stage, targets string
	)
	flag.IntVar(&batchSize, "batch", 10, "Maximum number of points to bundle in a single metrics update")
	flag.IntVar(&inN, "in", 1, "Number of inputs, counted left to right, all others will be considered outputs")
	flag.Float64Var(
		&errMargin,
		"margin",
		0,
		"Maximum difference between a prediction and the expected value for it to still be considered correct",
	)
	flag.BoolVar(&headers, "headers", false, "If true, the first line will be used to name the values")
	flag.DurationVar(&pc.Timeout, "timeout", 15*time.Second, "Maximum time to wait for the production of a message")
	flag.StringVar(&pc.Topic, "topic", "nerd-events", "Where to produce the messages when using Kafka")
	flag.StringVar(&pc.Type, "producer", "rest", "What producer to use. Supported values are rest and kafka")
	flag.StringVar(&sep, "sep", " ", "String sequence that denotes the end of one field and the start of the next")
	flag.StringVar(&seriesID, "series", "", "ID of the series that these points belong to")
	flag.StringVar(
		&stage,
		"stage",
		"test",
		"Category of the data, production for real world patterns, test for anything else",
	)
	flag.StringVar(
		&targets,
		"targets",
		"",
		"Comma separated list of protocol://host:port for nerd instances when using rest, host:port of Kafka brokers when using kafka",
	)
	flag.Parse()

	// Check arguments
	path := flag.Arg(0)
	if path == "" {
		fmt.Println("ERROR: No file path specified")
		os.Exit(1)
	}
	if seriesID == "" {
		fmt.Println("ERROR: No series specified")
		os.Exit(1)
	}
	if targets == "" {
		fmt.Println("ERROR: No targets specified")
		os.Exit(1)
	}

	// Initialize producer
	pc.Addresses = strings.Split(targets, ",")
	p, err := producer.New(pc)
	if err != nil {
		fmt.Println("ERROR: Failed to start producer (" + err.Error() + ")")
		os.Exit(1)
	}
	defer p.Close()
	fmt.Println(pc.Type + " producer initialized with targets: " + targets)

	// Initialize collector
	out := make(chan types.CategorizedPoint, 10)
	fc := NewFileCollector(headers, inN, out, path, sep)
	fmt.Println("file collector created for path: " + path)

	batch := []types.CategorizedPoint{}
	sent := 0
	go fc.Collect()
	fmt.Println("collection started")
	for point := range out {
		batch = append(batch, point)
		if len(batch) < batchSize {
			continue
		}
		mu := types.MetricsUpdate{
			SeriesID:  seriesID,
			ErrMargin: float32(errMargin),
			Points:    batch,
			Stage:     stage,
		}
		err = send(p, path, &mu)
		if err != nil {
			fmt.Println("ERROR: " + err.Error())
			os.Exit(1)
		}
		sent += len(batch)
		batch = []types.CategorizedPoint{}
	}
	if fc.Err() != nil {
		fmt.Println("ERROR: " + fc.Err().Error())
		os.Exit(1)
	}
	// Send any remaining points
	if len(batch) != 0 {
		mu := types.MetricsUpdate{
			SeriesID:  seriesID,
			ErrMargin: float32(errMargin),
			Points:    batch,
			Stage:     stage,
		}
		err = send(p, path, &mu)
		if err != nil {
			fmt.Println("ERROR: " + err.Error())
			os.Exit(1)
		}
		sent += len(batch)
	}
	fmt.Println("successfully produced " + strconv.Itoa(sent) + " points")
}

// send encapsulates the logic for wrapping a metrics update in a cloud event and sending it
func send(p producer.Producer, subject string, mu *types.MetricsUpdate) error {
	event := cloudevents.NewEvent()
	event.SetDataContentType("application/json")
	event.SetDataSchema("github.com/qvantel/nerd/api/types/")
	event.SetID(uuid.New().String())
	event.SetSource("fcollect")
	event.SetSubject(subject)
	event.SetType("com.qvantel.nerd.metricsupdate")

	event.SetData("application/json", mu)

	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.Send(mu.SeriesID, raw)
}
