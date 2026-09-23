// Package modeltrace implements ModelTrace's closed-set number fingerprint scorer.
// Derived from xqy2006/ModelTrace (MIT); see third_party/modeltrace/LICENSE.
package modeltrace

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"unicode"
)

//go:embed unified_bank.json
var bankJSON []byte

//go:embed source.json
var sourceJSON []byte

const MaxBankBytes = 4 * 1024 * 1024

type Source struct {
	Commit          string `json:"commit"`
	AlgorithmSHA256 string `json:"algorithm_sha256"`
	BankSHA256      string `json:"bank_sha256"`
}

func EmbeddedSource() Source {
	var source Source
	_ = json.Unmarshal(sourceJSON, &source)
	return source
}
func EmbeddedData() []byte { return append([]byte(nil), bankJSON...) }

const Dimension = 355

type Model struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Family      string    `json:"family"`
	FamilyName  string    `json:"family_name"`
	Counts      []float64 `json:"counts"`
}
type Artifact struct {
	Mean         []float64     `json:"feature_mean"`
	Scale        []float64     `json:"feature_scale"`
	Basis        [][]float64   `json:"nuisance_basis"`
	Centroids    [][]float64   `json:"centroids"`
	Environments [][][]float64 `json:"environment_centroids"`
	Weight       float64       `json:"weight"`
}
type Calibration struct {
	Beta       float64 `json:"beta"`
	CVAccuracy float64 `json:"cv_accuracy"`
}
type Bank struct {
	BuiltAt string  `json:"built_at"`
	Models  []Model `json:"models"`
	Robust  struct {
		ModelOrder []string `json:"model_order"`
		Hellinger  Artifact `json:"hellinger"`
		Ordered    Artifact `json:"ordered_blocks"`
	} `json:"robust"`
	Calibration map[string]Calibration `json:"calibration"`
	SHA256      string                 `json:"sha256"`
}
type Output struct {
	Text     string `json:"text"`
	Expected int    `json:"expected_count"`
}
type Candidate struct {
	Model       string  `json:"model"`
	Name        string  `json:"display_name"`
	Family      string  `json:"family"`
	Probability float64 `json:"probability"`
	Score       float64 `json:"score"`
	Similarity  float64 `json:"profile_similarity"`
}
type Diagnostic struct {
	Index    int  `json:"index"`
	Parsed   int  `json:"parsed_numbers"`
	Minimum  int  `json:"minimum_numbers"`
	Accepted bool `json:"accepted"`
}
type Result struct {
	Prediction          string             `json:"prediction"`
	Probability         float64            `json:"probability"`
	UsedOutputs         int                `json:"used_outputs"`
	Results             []Candidate        `json:"results"`
	Diagnostics         []Diagnostic       `json:"diagnostics"`
	FamilyProbabilities map[string]float64 `json:"family_probabilities"`
	BankSHA256          string             `json:"bank_sha256"`
	BankBuiltAt         string             `json:"bank_built_at"`
}

func EmbeddedBank() (*Bank, error) { return ParseBank(bankJSON) }

// ParseBank validates external data before it can reach the scoring code.
func ParseBank(raw []byte) (*Bank, error) {
	if len(raw) == 0 || len(raw) > MaxBankBytes {
		return nil, errors.New("invalid bank size")
	}
	var b Bank
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	b.SHA256 = hex.EncodeToString(sum[:])
	if len(b.Models) == 0 || len(b.Models) > 128 || len(b.BuiltAt) > 80 || len(b.Models) != len(b.Robust.ModelOrder) {
		return nil, errors.New("invalid embedded model order")
	}
	seen := map[string]bool{}
	for i, m := range b.Models {
		if m.ID == "" || len(m.ID) > 160 || len(m.DisplayName) > 200 || len(m.Family) > 80 || seen[m.ID] || m.ID != b.Robust.ModelOrder[i] || len(m.Counts) != Dimension {
			return nil, errors.New("invalid embedded model")
		}
		seen[m.ID] = true
		var total float64
		for _, n := range m.Counts {
			if !bounded(n) || n < 0 {
				return nil, errors.New("invalid model counts")
			}
			total += n
		}
		if total == 0 {
			return nil, errors.New("empty model counts")
		}
	}
	for _, v := range []struct {
		a   Artifact
		dim int
	}{{b.Robust.Hellinger, Dimension}, {b.Robust.Ordered, 74}} {
		if len(v.a.Basis) > v.dim || len(v.a.Environments) > 128 || !bounded(v.a.Weight) || v.a.Weight < 0 || v.a.Weight > 1 || len(v.a.Mean) != v.dim || len(v.a.Scale) != v.dim || len(v.a.Centroids) != len(b.Models) {
			return nil, errors.New("invalid embedded artifact")
		}
		for _, s := range v.a.Scale {
			if s < 1e-9 || !bounded(s) {
				return nil, errors.New("invalid feature scale")
			}
		}
		for _, row := range v.a.Basis {
			if dot(row, row) > 1.01 {
				return nil, errors.New("invalid nuisance basis norm")
			}
		}
		rows := append(append([][]float64{v.a.Mean}, v.a.Basis...), v.a.Centroids...)
		for _, e := range v.a.Environments {
			if len(e) != len(b.Models) {
				return nil, errors.New("invalid environment")
			}
			rows = append(rows, e...)
		}
		for _, r := range rows {
			if len(r) != v.dim {
				return nil, errors.New("invalid feature vector")
			}
			for _, n := range r {
				if !bounded(n) {
					return nil, errors.New("invalid feature value")
				}
			}
		}
	}
	for _, k := range []string{"1", "2", "3"} {
		if !bounded(b.Calibration[k].Beta) || b.Calibration[k].Beta <= 0 || b.Calibration[k].Beta > 1000 {
			return nil, errors.New("invalid calibration")
		}
	}
	if b.Robust.Ordered.Weight > 0 && len(b.Robust.Ordered.Environments) == 0 {
		return nil, errors.New("missing ordered environments")
	}
	return &b, nil
}
func bounded(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) && math.Abs(n) <= 1e9 }

var digitRE = regexp.MustCompile(`[0-9]+`)

func ParseNumbers(text string) []int {
	var best, current []int
	previous := 0
	for _, loc := range digitRE.FindAllStringIndex(text, -1) {
		hasLetter := false
		for _, r := range text[previous:loc[0]] {
			if unicode.IsLetter(r) {
				hasLetter = true
				break
			}
		}
		if len(current) > 0 && hasLetter {
			if len(current) > len(best) {
				best = current
			}
			current = nil
		}
		n, err := strconv.Atoi(text[loc[0]:loc[1]])
		if err == nil && n >= 1 && n <= Dimension {
			current = append(current, n)
		}
		previous = loc[1]
	}
	if len(current) > len(best) {
		best = current
	}
	return best
}
func Minimum(expected int) int { return max(80, int(math.Ceil(float64(expected)*.55))) }
func counts(numbers []int) []float64 {
	v := make([]float64, Dimension)
	for _, n := range numbers {
		v[n-1]++
	}
	return v
}
func dot(a, b []float64) float64 {
	var s float64
	for i, x := range a {
		s += x * b[i]
	}
	return s
}
func standard(v []float64) []float64 {
	var mean, variance float64
	for _, x := range v {
		mean += x
	}
	mean /= float64(len(v))
	for _, x := range v {
		variance += (x - mean) * (x - mean)
	}
	scale := math.Max(math.Sqrt(variance/float64(len(v))), 1e-12)
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = (x - mean) / scale
	}
	return out
}
func unit(v []float64) []float64 {
	n := math.Max(math.Sqrt(dot(v, v)), 1e-12)
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / n
	}
	return out
}

// Match the upstream browser scorer's sequential projection. Its learned bases
// are orthonormal; parity tests cover the resulting numerical tolerance.
func project(v []float64, basis [][]float64) []float64 {
	out := append([]float64{}, v...)
	for _, b := range basis {
		p := dot(out, b)
		for i := range out {
			out[i] -= p * b[i]
		}
	}
	return out
}
func featureScale(v []float64, a Artifact) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = (x - a.Mean[i]) / a.Scale[i]
	}
	return out
}
func smoothed(v []float64) []float64 {
	var total float64
	for _, x := range v {
		total += x + .5
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = math.Sqrt((x + .5) / total)
	}
	return out
}
func similarities(v []float64, centroids [][]float64) []float64 {
	out := make([]float64, len(centroids))
	for i, c := range centroids {
		out[i] = dot(v, c)
	}
	return out
}
func orderedFeature(numbers []int) []float64 {
	var out []float64
	start := 0
	for block := 0; block < 4; block++ {
		size := len(numbers) / 4
		if block < len(numbers)%4 {
			size++
		}
		bins := make([]float64, 16)
		for _, n := range numbers[start : start+size] {
			idx := min(15, (n-1)*16/355)
			bins[idx]++
		}
		out = append(out, smoothed(bins)...)
		start += size
	}
	digits := make([]float64, 10)
	for _, n := range numbers {
		digits[n%10]++
	}
	return append(out, smoothed(digits)...)
}
func (b *Bank) score(numbers []int) []float64 {
	h := b.Robust.Hellinger
	marginal := standard(similarities(unit(project(featureScale(smoothed(counts(numbers)), h), h.Basis)), h.Centroids))
	a := b.Robust.Ordered
	if a.Weight == 0 {
		return marginal
	}
	scaled := featureScale(orderedFeature(numbers), a)
	normalized := unit(scaled)
	templates := make([]float64, len(b.Models))
	for i := range templates {
		templates[i] = math.Inf(-1)
	}
	for _, env := range a.Environments {
		for i, c := range env {
			templates[i] = math.Max(templates[i], dot(normalized, c))
		}
	}
	templates = standard(templates)
	nuisance := standard(similarities(unit(project(scaled, a.Basis)), a.Centroids))
	for i := range nuisance {
		nuisance[i] = .5*templates[i] + .5*nuisance[i]
	}
	ordered := standard(nuisance)
	for i := range marginal {
		marginal[i] = (1-a.Weight)*marginal[i] + a.Weight*ordered[i]
	}
	return marginal
}
func similarity(p, q []float64) float64 {
	var pt, qt float64
	for _, x := range p {
		pt += x
	}
	for _, x := range q {
		qt += x + .5
	}
	var divergence float64
	for i := range p {
		a, c := p[i]/pt, (q[i]+.5)/qt
		m := (a + c) / 2
		if a > 0 {
			divergence += a * math.Log(a/m) / 2
		}
		divergence += c * math.Log(c/m) / 2
	}
	return 1 - math.Sqrt(math.Max(0, divergence)/math.Log(2))
}
func (b *Bank) Contains(model string) bool {
	for _, m := range b.Models {
		if m.ID == model {
			return true
		}
	}
	return false
}
func (b *Bank) Analyze(outputs []Output) (*Result, error) {
	if len(outputs) < 1 || len(outputs) > 3 {
		return nil, errors.New("需要 1–3 份回答")
	}
	r := &Result{BankSHA256: b.SHA256, BankBuiltAt: b.BuiltAt, FamilyProbabilities: map[string]float64{}}
	scores := make([]float64, len(b.Models))
	pooled := make([]float64, Dimension)
	for i, o := range outputs {
		if len(o.Text) > 128*1024 || o.Expected < 0 || o.Expected > 1000 {
			return nil, fmt.Errorf("回答 %d 超出限制", i+1)
		}
		n := ParseNumbers(o.Text)
		d := Diagnostic{i, len(n), Minimum(o.Expected), len(n) >= Minimum(o.Expected)}
		r.Diagnostics = append(r.Diagnostics, d)
		if !d.Accepted {
			continue
		}
		r.UsedOutputs++
		s := b.score(n)
		for j, x := range s {
			scores[j] += x
		}
		for j, x := range counts(n) {
			pooled[j] += x
		}
	}
	if r.UsedOutputs == 0 {
		return nil, errors.New("没有可用回答：数字不足或回答被截断")
	}
	beta := b.Calibration[strconv.Itoa(r.UsedOutputs)].Beta
	maximum := math.Inf(-1)
	for i := range scores {
		scores[i] /= float64(r.UsedOutputs)
		maximum = math.Max(maximum, scores[i]*beta)
	}
	weights := make([]float64, len(scores))
	var total float64
	for i, s := range scores {
		weights[i] = math.Exp(s*beta - maximum)
		total += weights[i]
	}
	for i, m := range b.Models {
		p := weights[i] / total
		r.Results = append(r.Results, Candidate{m.ID, m.DisplayName, m.Family, p, scores[i], similarity(pooled, m.Counts)})
		r.FamilyProbabilities[m.Family] += p
	}
	sort.SliceStable(r.Results, func(i, j int) bool { return r.Results[i].Probability > r.Results[j].Probability })
	r.Prediction = r.Results[0].Model
	r.Probability = r.Results[0].Probability
	return r, nil
}
