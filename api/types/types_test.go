package types

import "testing"

func TestHasChanged(t *testing.T) {
	cp1 := CategorizedPoint{
		Inputs:    map[string]float32{"subs": 42},
		Outputs:   map[string]float32{"size": 1.5},
		TimeStamp: int64(777808800),
	}
	cp2 := CategorizedPoint{
		Inputs:    map[string]float32{"subs": 42},
		Outputs:   map[string]float32{"size": 1.5},
		TimeStamp: int64(777808801),
	}
	cp3 := CategorizedPoint{
		Inputs:    map[string]float32{"subs": 42},
		Outputs:   map[string]float32{"size": 3},
		TimeStamp: int64(777808802),
	}
	cp4 := CategorizedPoint{
		Inputs:    map[string]float32{"subs": 84},
		Outputs:   map[string]float32{"size": 1.5},
		TimeStamp: int64(777808803),
	}
	cp5 := CategorizedPoint{
		Inputs:    map[string]float32{"subs": 84},
		Outputs:   map[string]float32{"size": 3},
		TimeStamp: int64(777808804),
	}
	if cp1.HasChanged(cp1) || cp2.HasChanged(cp1) {
		t.Errorf("HasChanged should return false when no inputs or outputs change")
	}
	if cp3.HasChanged(cp1) || cp4.HasChanged(cp1) {
		t.Errorf("HasChanged should return false when only inputs or outputs change")
	}
	if !cp5.HasChanged(cp1) {
		t.Errorf("HasChanged should return true when at least one input and one output change")
	}
	cp6 := CategorizedPoint{
		Inputs:    map[string]float32{"subs": 42, "static": 1},
		Outputs:   map[string]float32{"size": 1.5, "static": 1},
		TimeStamp: int64(777808805),
	}
	cp7 := CategorizedPoint{
		Inputs:    map[string]float32{"subs": 84, "static": 1},
		Outputs:   map[string]float32{"size": 3, "static": 1},
		TimeStamp: int64(777808806),
	}
	if !cp7.HasChanged(cp6) {
		t.Errorf("HasChanged should return true when at least one input and one output change")
	}
}
