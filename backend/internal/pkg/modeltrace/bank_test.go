package modeltrace

import (
	"encoding/json"
	"testing"
)

func TestEmbeddedSourceMetadata(t *testing.T) {
	bank, err := EmbeddedBank()
	if err != nil {
		t.Fatal(err)
	}
	source := EmbeddedSource()
	if source.BankSHA256 != bank.SHA256 || len(source.Commit) != 40 || len(source.AlgorithmSHA256) != 64 {
		t.Fatal("embedded source metadata does not match bank")
	}
}
func TestExternalBankValidation(t *testing.T) {
	for _, change := range []func(*Bank){
		func(b *Bank) { b.Models[1].ID = b.Models[0].ID },
		func(b *Bank) { b.Models[0].Counts[0] = -1 },
		func(b *Bank) { b.Robust.Ordered.Environments = nil },
		func(b *Bank) { b.Robust.Ordered.Scale[0] = 1e-100 },
		func(b *Bank) { b.Robust.Hellinger.Basis[0][0] = 1e9 },
		func(b *Bank) { b.Robust.Ordered.Environments[0][0] = []float64{1} },
		func(b *Bank) { b.Calibration["3"] = Calibration{} },
	} {
		b, err := EmbeddedBank()
		if err != nil {
			t.Fatal(err)
		}
		change(b)
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = ParseBank(raw); err == nil {
			t.Fatal("invalid external bank accepted")
		}
	}
}
