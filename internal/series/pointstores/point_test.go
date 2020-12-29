package pointstores

import (
	"encoding/json"
	"testing"
)

func TestID(t *testing.T) {
	sec := int64(777808800)
	a := Point{map[string]string{"program": "qvantel", "env": "sphere2"}, map[string]float32{"storage": 234.23}, sec}
	b := Point{map[string]string{"env": "sphere2", "program": "qvantel"}, map[string]float32{"storage": 42.1}, sec}

	firstA := a.ID()
	firstB := b.ID()

	if firstA != firstB {
		t.Errorf("Two points with the same labels and timestamp didn't return the same id")
	}

	secondB := b.ID()
	if firstB != secondB {
		t.Errorf("Generating an ID for the same point must allways return the same value")
	}

	c := Point{map[string]string{"program": "qvantel"}, map[string]float32{"storage": 42.1}, sec}
	if firstA == c.ID() {
		t.Errorf("The ID hash should be taking all label values into account")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	sec := int64(777808800)
	a := Point{map[string]string{"program": "qvantel", "env": "sphere2"}, map[string]float32{"storage": 234.23}, sec}

	jPoint, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("The custom JSON marshal method shouldn't fail to convert a known valid point (%s)", err.Error())
	}

	var b Point
	err = json.Unmarshal(jPoint, &b)
	if err != nil {
		t.Fatalf("The custom JSON unmarshal method shouldn't fail to convert a known valid point (%s)", err.Error())
	}
	if a.ID() != b.ID() {
		t.Errorf("The result from unmarshalling a marshalled point should be an identical object")
	}
}
