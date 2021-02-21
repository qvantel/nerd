// +build functional

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/qvantel/nerd/api/types"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	nerd    testcontainers.Container
	nerdURL string
)

func eval(id string, inputs map[string]float32) (map[string]float32, error) {
	raw, _ := json.Marshal(inputs)
	resp, err := http.Post(nerdURL+"/api/v1/nets/"+id+"/evaluate", "application/json", bytes.NewBuffer(raw))
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var outputs map[string]float32
	err = json.Unmarshal(body, &outputs)
	if err != nil {
		return nil, err
	}
	return outputs, nil
}

func startNerd(ctx context.Context) (err error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{Context: "../"},
		ExposedPorts:   []string{"5400/tcp"},
		WaitingFor:     wait.ForHTTP("/").WithPort("5400/tcp"),
	}
	nerd, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return err
	}
	nerdURL, err = nerd.Endpoint(ctx, "")
	if err != nil {
		return err
	}
	nerdURL = "http://" + nerdURL
	return nil
}

func train(tr types.TrainRequest) error {
	raw, _ := json.Marshal(tr)
	resp, err := http.Post(nerdURL+"/api/v1/nets", "application/json", bytes.NewBuffer(raw))
	if err != nil {
		return err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusAccepted {
		return errors.New(resp.Status)
	}
	return nil
}

func TestQuickStart(t *testing.T) {
	ctx := context.Background()

	// Load test set
	err := nerd.CopyFileToContainer(ctx, "../test/shuffled_banknote_authentication.txt", "/dataset.txt", 0777)
	if err != nil {
		t.Fatalf("Failed to copy test set into the container (%s)", err.Error())
	}
	exitCode, err := nerd.Exec(ctx, []string{
		"/opt/docker/fcollect",
		"-batch", "50",
		"-in", "4",
		"-margin", "0.4999999",
		"-sep", ",",
		"-series", "banknote-forgery-detection",
		"-targets", "http://localhost:5400",
		"/dataset.txt",
	})
	if err != nil {
		t.Fatalf("Failed to run fcollect (%s)", err.Error())
	}
	if exitCode != 0 {
		t.Fatalf("fcollect returned an error exit code (%d)", exitCode)
	}

	// Trigger training
	tr := types.TrainRequest{
		ErrMargin: 0.4999999,
		Inputs:    []string{"value-0", "value-1", "value-2", "value-3"},
		Outputs:   []string{"value-4"},
		Required:  1372,
		SeriesID:  "banknote-forgery-detection",
	}
	err = train(tr)
	if err != nil {
		t.Fatalf("Failed to train the series (%s)", err.Error())
	}
	time.Sleep(1 * time.Second)

	// Evaluate points
	id := "banknote-forgery-detection-8921e4a37dabacc06fec3318e908d9fe4eb75b46-7804b6fc74b5c0a74cc0820420fa0edf6b1a117c-mlp"
	out, err := eval(
		id,
		map[string]float32{
			"value-0": 3.2403,
			"value-1": -3.7082,
			"value-2": 5.2804,
			"value-3": 0.41291,
		},
	)
	if err != nil {
		t.Fatalf("Failed to evaluate the data of an authentic note (%s)", err.Error())
	}
	if math.Abs(1-float64(out["value-4"])) < math.Abs(0-float64(out["value-4"])) {
		t.Errorf("Output should have been closer to 0 than 1, got %f", out["value-4"])
	}
	out, err = eval(
		id,
		map[string]float32{
			"value-0": -1.4377,
			"value-1": -1.432,
			"value-2": 2.1144,
			"value-3": 0.42067,
		},
	)
	if err != nil {
		t.Fatalf("Failed to evaluate the data of a forged note (%s)", err.Error())
	}
	if math.Abs(1-float64(out["value-4"])) > math.Abs(0-float64(out["value-4"])) {
		t.Errorf("Output should have been closer to 1 than 0, got %f", out["value-4"])
	}
}

func TestMain(m *testing.M) {
	// Setup
	ctx := context.Background()
	err := startNerd(ctx)
	if err != nil {
		fmt.Printf("Error starting test nerd container (%s)", err.Error())
		os.Exit(1)
	}
	// Run
	code := m.Run()
	// Teardown
	if err == nil {
		nerd.Terminate(ctx)
	}
	os.Exit(code)
}
