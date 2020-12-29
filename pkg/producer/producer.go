// Package producer exposes common logic that all Go collectors can use for producing nerd events
package producer

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"time"
)

// ipPort represents a valid ipv4 address with format ip:port
var ipPort = regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?):\d{1,5}$`)

// Producer is an abstraction over how the service sends metrics updates to nerd
type Producer interface {
	Send(seriesID string, event []byte) error
	Close()
}

// Config holds the necessary configuration to set up a producer
type Config struct {
	Addresses []string      `json:"addresses"`
	Timeout   time.Duration `json:"timeout"`
	Topic     string        `json:"topic"`
	Type      string        `json:"type"`
}

// New returns a producer of the type selected in the configuration
func New(conf Config) (Producer, error) {
	switch conf.Type {
	case "kafka":
		return NewKafkaProducer(conf)
	case "rest":
		return NewRestProducer(conf)
	default:
		return nil, errors.New(conf.Type + " is not a valid producer type")
	}
}

// oneUp is used to check if at least one of the given addresses is usable. Only endpoints that use TCP are supported
func oneUp(addrs []string, timeout time.Duration) bool {
	for _, addr := range addrs {
		clean, err := cleanHostPort(addr)
		if err != nil {
			continue
		}
		d := net.Dialer{Timeout: timeout}
		conn, err := d.Dial("tcp", clean)
		if err != nil {
			continue
		}
		conn.Close()
		return true
	}
	return false
}

// cleanHostPort is needed so that invalid addresses can be detected quickly and so that we can check the connection to
// addresses with a scheme in them
func cleanHostPort(addr string) (string, error) {
	if ipPort.MatchString(addr) {
		return addr, nil
	}
	u, err := url.Parse(addr)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "https":
		clean := addr[8:]
		if u.Port() == "" {
			clean += ":443"
		}
		return clean, nil
	case "http":
		clean := addr[7:]
		if u.Port() == "" {
			clean += ":80"
		}
		return clean, nil
	default:
		return addr, nil
	}
}
