package main

import (
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

func TestValidateConfigWorks(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		err := ValidateConfig([]byte(
			"{\"policies\":[{\"role\":\"admin\", \"policy\":[{\"column\":\"Region\", \"values\":[\"one\",\"two\"]}]}]}",
		))
		if err != nil {
			t.Fail()
		}
	})
	t.Run("Valid config file", func(t *testing.T) {
		err := ValidateConfigFile("testdata/valid_policy_set.json")
		if err != nil {
			t.Fail()
		}
	})

	t.Run("Empty set of policies is ok", func(t *testing.T) {
		err := ValidateConfig([]byte("{\"policies\":[]}"))
		if err != nil {
			t.Fail()
		}
	})
}

func TestValidateConfigFails(t *testing.T) {
	t.Run("Invalid config file", func(t *testing.T) {
		err := ValidateConfigFile("testdata/invalid_policy_set.json")
		if err == nil {
			t.Fail()
		}
	})
	t.Run("Invalid config", func(t *testing.T) {
		err := ValidateConfig([]byte(
			"{\"policies\":[{\"oops\":\"admin\", \"policy_items\":[{\"column\":\"Region\", \"values\":[\"one\",\"two\"]}]}]}",
		))
		if err == nil {
			t.Fail()
		}
	})
}
