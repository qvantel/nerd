package config

import "testing"

func TestGetenv(t *testing.T) {
	got := Getenv("VAR_THAT_DOES_NOT_EXIST", "default value")
	if got != "default value" {
		t.Errorf("Getenv(\"VAR_THAT_DOES_NOT_EXIST\", \"default value\") = %s; want default value", got)
	}
}
