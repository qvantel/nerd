package main

import (
	"testing"

	"github.com/qvantel/nerd/api/types"
)

func TestCollect(t *testing.T) {
	out := make(chan types.CategorizedPoint, 10)
	fc := NewFileCollector(false, 9, out, "../../test/normalization_test_data.txt", " ")
	points := []types.CategorizedPoint{}

	go fc.Collect()
	for point := range out {
		points = append(points, point)
	}
	if fc.Err() != nil {
		t.Fatalf("Failed to collect data from file (%s)", fc.Err().Error())
	}

	if len(points) != 699 {
		t.Errorf("Expected to extract 699 points, got: %d instead", len(points))
	}
}
