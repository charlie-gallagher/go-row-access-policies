package main

import (
	"encoding/json"
	"testing"
)

func TestPolicyItemConvertsToJson(t *testing.T) {
	pi := PolicyItem{Column: "charlie", Values: []string{"one", "two", "three"}}
	got := pi.ToJson()
	want := "{\"column\":\"charlie\",\"values\":[\"one\",\"two\",\"three\"]}"

	if got != want {
		t.Fail()
	}
}

func TestConfigCanUnmarshal(t *testing.T) {
	data := []byte("{\"policies\":[{\"role\":\"admin\", \"policy\":[{\"column\":\"Region\", \"values\":[\"one\",\"two\"]}]}]}")
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fail()
	}
}

