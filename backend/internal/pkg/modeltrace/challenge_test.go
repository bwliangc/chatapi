package modeltrace

import "testing"

func TestClosedSetVerdict(t *testing.T) {
	bank, err := EmbeddedBank()
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.Model = "unknown-model"
	result := &Result{Prediction: bank.Models[0].ID, Probability: .9999, UsedOutputs: 3, Results: []Candidate{{Model: bank.Models[0].ID, Probability: .9999}, {Model: bank.Models[1].ID, Probability: .0001}}}
	if got := Classify(bank, config, result); got.Status != "unsupported" {
		t.Fatal(got)
	}
	config.Model = bank.Models[0].ID
	result.UsedOutputs = 2
	if got := Classify(bank, config, result); got.Status != "insufficient" {
		t.Fatal(got)
	}
}
func TestIndependentChallenges(t *testing.T) {
	challenges, err := Challenges(6)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, challenge := range challenges {
		if seen[challenge.Expected] || challenge.Expected < 292 || challenge.Expected > 332 {
			t.Fatal("invalid challenge sequence")
		}
		seen[challenge.Expected] = true
	}
}
