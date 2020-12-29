package producer

import "testing"

func TestCleanHostPort(t *testing.T) {
	addrs := [][]string{
		{"https://qvantel.com", "qvantel.com:443"},
		{"http://qvantel.com", "qvantel.com:80"},
		{"http://qvantel.com:5400", "qvantel.com:5400"},
		{"qvantel.com:5400", "qvantel.com:5400"},
		{"127.0.0.1:5400", "127.0.0.1:5400"},
		{"https://127.0.0.1:5400", "127.0.0.1:5400"},
		{"http://127.0.0.1:5400", "127.0.0.1:5400"},
	}
	for _, addr := range addrs {
		res, err := cleanHostPort(addr[0])
		if err != nil {
			t.Errorf("Failed to parse %s (%s)", addr[0], err.Error())
			continue
		}
		if res != addr[1] {
			t.Errorf("Wanted %s, got %s", addr[1], res)
			continue
		}
	}
}
