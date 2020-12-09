package pointstores

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
)

// Point represents a single measurement in a time series
type Point struct {
	Labels    map[string]string
	Values    map[string]float32
	TimeStamp int64
}

// ID generates a string that uniquely identifies a point. Useful for deduplication
func (p Point) ID() string {
	// We have to sort the labels in the map to ensure the hash is deterministic
	keys := make([]string, 0, len(p.Labels))
	for k := range p.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	hash := sha256.New()
	hash.Write([]byte(strconv.FormatInt(p.TimeStamp, 10)))
	for _, key := range keys {
		hash.Write([]byte(p.Labels[key]))
	}

	raw := hash.Sum(nil)
	id := make([]byte, hex.EncodedLen(len(raw)))
	hex.Encode(id, raw)

	return string(id)
}

// MarshalJSON is required for flattening the struct
func (p Point) MarshalJSON() ([]byte, error) {
	total := make(map[string]interface{}, 1+len(p.Labels)+len(p.Values))

	total["@timestamp"] = p.TimeStamp
	for label, value := range p.Labels {
		total[label] = value
	}
	for name, value := range p.Values {
		total[name] = value
	}

	jPoint, err := json.Marshal(total)
	return jPoint, err
}

// UnmarshalJSON makes sure that we can recover a point struct from a flattened representation
func (p *Point) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}
	p.Labels = map[string]string{}
	p.Values = map[string]float32{}
	for key, value := range raw {
		if key == "@timestamp" {
			p.TimeStamp = int64(value.(float64))
			continue
		}
		switch value.(type) {
		case float64:
			p.Values[key] = float32(value.(float64))
		case string:
			p.Labels[key] = value.(string)
		}
	}
	return nil
}
