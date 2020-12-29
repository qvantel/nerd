// Package types contains most of the objects that the API reads or writes
package types

// PagedRes is a wrapper for a paged response where next can be provided as offset for the subsequent request and last
// can be used to determine when there is nothing left to read
type PagedRes struct {
	Last    bool        `json:"last"`
	Next    int         `json:"next"`
	Results interface{} `json:"results"`
}

// SimpleRes is used for errors and those cases where the response code would be sufficient but a JSON response helps
// consistency and user friendliness
type SimpleRes struct {
	Result string `json:"result"` // Possible values are "error" and "ok"
	Msg    string `json:"message"`
}

// NewOkRes is a shortcut for building a SimpleRes for a successful result
func NewOkRes(msg string) *SimpleRes {
	return &SimpleRes{Result: "ok", Msg: msg}
}

// NewErrorRes is a shortcut for building a SimpleRes for a failed result
func NewErrorRes(msg string) *SimpleRes {
	return &SimpleRes{Result: "error", Msg: msg}
}

// BriefNet is a lightweight and standardized representation for neural network parameters
type BriefNet struct {
	Accuracy   float32            `json:"accuracy" example:"0.9"` // Fraction of patterns that were predicted correctly during testing
	Averages   map[string]float32 `json:"averages"`               // Averages of each value in the patterns that were used for training
	Deviations map[string]float32 `json:"deviations"`             // Standard deviation of each value in the patterns that were used for training
	ErrMargin  float32            `json:"errMargin"`              // Maximum difference between the expected and produced result to still be considered correct during testing
	ID         string             `json:"id"`
	Inputs     []string           `json:"inputs"`
	Outputs    []string           `json:"outputs"`
	Type       string             `json:"type"`
}

// TrainRequest as its name implies, is used to ask the training service to create or update a net
type TrainRequest struct {
	ErrMargin float32  `json:"errMargin"` // Maximum difference between the expected and produced result to still be considered correct during testing
	Inputs    []string `json:"inputs"`    // Which of the series values should be treated as inputs
	Outputs   []string `json:"outputs"`   // Which of the series values should be treated as outputs
	Required  int      `json:"required"`  // Number of points from the series that should be used to train and test
	SeriesID  string   `json:"seriesID"`
}

// BriefSeries is a lightweight representation of a time series
type BriefSeries struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// CategorizedPoint is similar to pointstores.Point but has its values divided into inputs and outputs (this separation
// isn't important when it comes to storing it but allows producers to state their intentions so that, when enough
// points are available, a training request can be automatically generated)
type CategorizedPoint struct {
	Inputs    map[string]float32 `json:"inputs"`
	Outputs   map[string]float32 `json:"outputs"`
	TimeStamp int64              `json:"timestamp"`
}

// HasChanged checks that at least one input and one output is different from the given point. As we're going to be
// using these to train a neural network it's important that we don't effectively just train it to recognize one value
// very well or that the inputs have 0 influence on the outputs
func (cp *CategorizedPoint) HasChanged(other CategorizedPoint) bool {
	inDiff := false
	for label, value := range cp.Inputs {
		if other.Inputs[label] != value {
			inDiff = true
			break
		}
	}
	outDiff := false
	for label, value := range cp.Outputs {
		if other.Outputs[label] != value {
			outDiff = true
			break
		}
	}
	return inDiff && outDiff
}

// MetricsUpdate bundles snapshots of a set of values that share some relation that could be used to predict each
// other in a system. This relation doesn't need to be understood (otherwise implement a specific tool for it) but it
// should be there for everything to work
type MetricsUpdate struct {
	SeriesID  string             `json:"seriesID"`
	ErrMargin float32            `json:"errMargin"` // How much the predicted value of the net can differ from the actual but still be considered acceptable
	Labels    map[string]string  `json:"labels"`
	Points    []CategorizedPoint `json:"points"`
	Stage     string             `json:"stage"` // Valid stages are test or production
}
