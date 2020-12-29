package producer

import (
	"bytes"
	"crypto/tls"
	"errors"
	"math/rand"
	"net/http"
	"time"
)

// RestProducer is a Producer implementation for sending events directly to nerd through its API. Its use is discouraged
// when pairing it with a production nerd setup given that it doesn't support load balancing by series ID
type RestProducer struct {
	endpoints []string
	client    *http.Client
}

// NewRestProducer checks the provided addresses and creates a rest producer
func NewRestProducer(conf Config) (*RestProducer, error) {
	if !oneUp(conf.Addresses, conf.Timeout) {
		return nil, errors.New("none of the provided nerd endpoints are usable")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &RestProducer{
		endpoints: conf.Addresses,
		client:    &http.Client{Transport: tr, Timeout: conf.Timeout},
	}, nil
}

// Close is required to be defined to comply with the Producer interface but not really needed for rest
func (rp *RestProducer) Close() {
	return
}

// Send posts the given event to one of the defined addresses picked at random
func (rp *RestProducer) Send(seriesID string, event []byte) error {
	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(len(rp.endpoints))
	url := rp.endpoints[i] + "/api/v1/series/process"
	resp, err := rp.client.Post(url, "application/json", bytes.NewBuffer(event))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted {
		return errors.New("received http status " + resp.Status)
	}
	return nil
}
