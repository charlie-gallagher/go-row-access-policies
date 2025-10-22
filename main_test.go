package main

import (
	// "encoding/json"
	"testing"
)

func TestPolicyItemConvertsToJson(t *testing.T) {
	t.Run("empty policy item", func(t *testing.T) {
		pi := PolicyItem{}
		got := pi.ToJson()
		want := "null"
		if got != want {
			t.Fail()
		}
	})
	t.Run("policy item with no values", func(t *testing.T) {
		pi := PolicyItem{Column: "charlie", Values: []string{}}
		got := pi.ToJson()
		want := "null"
		if got != want {
			t.Fail()
		}
	})
	t.Run("non-empty policy item", func(t *testing.T) {
		pi := PolicyItem{Column: "charlie", Values: []string{"one", "two", "three"}}
		got := pi.ToJson()
		want := "{\"column\":\"charlie\",\"values\":[\"one\",\"two\",\"three\"]}"
		if got != want {
			t.Fail()
		}
	})
}

func TestPolicyConvertsToJson(t *testing.T) {
	t.Run("empty policy", func(t *testing.T) {
		p := Policy{}
		got := p.ToJson()
		want := "null"
		if got != want {
			t.Fail()
		}
	})
	t.Run("non-empty policy", func(t *testing.T) {
		p := Policy{Role: "admin", Policy: []PolicyItem{{Column: "Region", Values: []string{"one", "two", "three"}}}}
		got := p.ToJson()
		want := "{\"role\":\"admin\",\"policy\":[{\"column\":\"Region\",\"values\":[\"one\",\"two\",\"three\"]}]}"
		if got != want {
			t.Fail()
		}
	})
}

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		err := ValidateConfig("testdata/valid_simple_policy.json")
		if err != nil {
			t.Fail()
		}
	})
}

// func TestConfigCanUnmarshal(t *testing.T) {
// 	data := []byte("{\"policies\":[{\"role\":\"admin\", \"policy\":[{\"column\":\"Region\", \"values\":[\"one\",\"two\"]}]}]}")
// 	ps, err := LoadRolePolicies(data)
// 	if err != nil {
// 		t.Fail()
// 	}
// 	if len(ps.Policies) != 1 {
// 		t.Fail()
// 	}
// 	if ps.Policies[0].Role != "admin" {
// 		t.Fail()
// 	}
// 	if len(ps.Policies[0].Policy) != 2 {
// 		t.Fail()
// 	}
// 	if ps.Policies[0].Policy[0].Column != "Region" {
// 		t.Fail()
// 	}
// 	if len(ps.Policies[0].Policy[0].Values) != 2 {
// 		t.Fail()
// 	}
// }
