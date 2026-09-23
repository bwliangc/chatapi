package modeltrace

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamParity(t *testing.T) {
	bank, err := EmbeddedBank()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name     string   `json:"name"`
		Outputs  []Output `json:"outputs"`
		Expected struct {
			Prediction  string       `json:"prediction"`
			UsedOutputs int          `json:"used_outputs"`
			Results     []Candidate  `json:"results"`
			Diagnostics []Diagnostic `json:"diagnostics"`
			Families    []struct {
				Family      string  `json:"family"`
				Probability float64 `json:"probability"`
			} `json:"family_probabilities"`
		} `json:"expected"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			got, err := bank.Analyze(c.Outputs)
			if err != nil {
				t.Fatal(err)
			}
			if got.Prediction != c.Expected.Prediction || got.UsedOutputs != c.Expected.UsedOutputs {
				t.Fatalf("mismatch: %+v", got)
			}
			if !reflect.DeepEqual(got.Diagnostics, c.Expected.Diagnostics) {
				t.Fatalf("diagnostics mismatch")
			}
			if len(got.Results) != len(c.Expected.Results) {
				t.Fatal("candidate count mismatch")
			}
			for _, family := range c.Expected.Families {
				if math.Abs(got.FamilyProbabilities[family.Family]-family.Probability) > 1e-9 {
					t.Fatal("family probability mismatch")
				}
			}
			for i, v := range got.Results {
				want := c.Expected.Results[i]
				if v.Model != want.Model || math.Abs(v.Probability-want.Probability) > 1e-9 || math.Abs(v.Score-want.Score) > 1e-9 || math.Abs(v.Similarity-want.Similarity) > 1e-9 {
					t.Fatalf("candidate mismatch: %+v vs %+v", v, want)
				}
			}
		})
	}
}
func TestParseNumbersLongestRun(t *testing.T) {
	for _, c := range []struct {
		input string
		want  []int
	}{{"说明 3 个数字：1, 355, 356, 0, 2 后续 7", []int{1, 355, 2}}, {"1,2 abc 3,4", []int{1, 2}}, {"99999999999999999999999999999999 1 2", []int{1, 2}}} {
		if got := ParseNumbers(c.input); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%q: got %v want %v", c.input, got, c.want)
		}
	}
}
func TestInvalidOutputs(t *testing.T) {
	bank, _ := EmbeddedBank()
	for _, v := range [][]Output{nil, {{Text: "1 2 3", Expected: 300}}, make([]Output, 4), {{Text: "123", Expected: -1}}} {
		if _, err := bank.Analyze(v); err == nil {
			t.Fatal("accepted invalid output")
		}
	}
}
