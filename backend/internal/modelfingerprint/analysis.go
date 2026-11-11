// Package modelfingerprint implements the lmfpd shared-detector-v2 numerical
// fingerprint ranking. Ported from lmfpd-sdk 0.2.0 (MIT, see LICENSE).
package modelfingerprint

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"unicode"
)

type Output struct {
	Text     string `json:"text"`
	Expected int    `json:"expected_count"`
}
type Candidate struct {
	Model       string   `json:"model"`
	DisplayName string   `json:"display_name"`
	Family      string   `json:"family"`
	Score       float64  `json:"score"`
	Probability *float64 `json:"probability"`
}
type Analysis struct {
	DetectorSHA256 string      `json:"detector_sha256"`
	Decision       string      `json:"decision"`
	Prediction     string      `json:"prediction"`
	Used           int         `json:"used_outputs"`
	Results        []Candidate `json:"results"`
	Method         string      `json:"method"`
	BankVersion    string      `json:"bank_version"`
	Reason         string      `json:"reason"`
}
type Model struct {
	ID            string `json:"id"`
	DisplayName   string `json:"display_name"`
	Family        string `json:"family"`
	ResponseCount int    `json:"response_count"`
}
type Bank struct {
	Schema  string  `json:"schema"`
	BuiltAt string  `json:"built_at"`
	Models  []Model `json:"models"`
	Files   map[string]struct {
		SHA256 string   `json:"sha256"`
		Parts  []string `json:"parts"`
	} `json:"files"`
}
type Params struct {
	Mean  []float64 `json:"mean"`
	Scale []float64 `json:"scale"`
}
type BaselinePart struct {
	Mean         []float64     `json:"feature_mean"`
	Scale        []float64     `json:"feature_scale"`
	Nuisance     [][]float64   `json:"nuisance_basis"`
	Centroids    [][]float64   `json:"centroids"`
	Environments [][][]float64 `json:"environment_centroids"`
}
type Artifact struct {
	Schema        string   `json:"schema"`
	BuiltAt       string   `json:"bank_built_at"`
	IDs           []string `json:"model_ids"`
	Counts        []int    `json:"response_counts"`
	BaseHash      string   `json:"base_sha256"`
	ReferenceHash string   `json:"source_reference_sha256"`
	Ranker        struct {
		Full       []Params      `json:"full_params"`
		Head       []Params      `json:"head_params"`
		LDA        [][]float64   `json:"lda_weights"`
		Bias       []float64     `json:"lda_bias"`
		References [][][]float64 `json:"references"`
		Bank       struct {
			H BaselinePart `json:"hellinger"`
			O BaselinePart `json:"ordered_blocks"`
		} `json:"bank"`
		Positional struct {
			Params  []Params    `json:"params"`
			Weights [][]float64 `json:"weights"`
			Bias    []float64   `json:"bias"`
		} `json:"positional"`
		Blend struct {
			Frequency  float64 `json:"frequency"`
			Positional float64 `json:"positional"`
		} `json:"blend"`
	} `json:"ranker"`
	Calibration *struct {
		Schema  string  `json:"schema"`
		Method  string  `json:"method"`
		Tau     float64 `json:"tau"`
		Binding struct {
			BaseHash      string   `json:"base_sha256"`
			ReferenceHash string   `json:"reference_sha256"`
			IDs           []string `json:"model_ids"`
		} `json:"binding"`
	} `json:"calibration"`
}

var digits = regexp.MustCompile(`\d+`)

func ParseNumbers(text string) []float64 {
	var best, current []float64
	end := 0
	for _, m := range digits.FindAllStringIndex(text, -1) {
		letters := false
		for _, r := range text[end:m[0]] {
			if unicode.IsLetter(r) {
				letters = true
				break
			}
		}
		if len(current) > 0 && letters {
			if len(current) > len(best) {
				best = current
			}
			current = nil
		}
		value, _ := strconv.Atoi(text[m[0]:m[1]])
		if value >= 1 && value <= 355 {
			current = append(current, float64(value))
		}
		end = m[1]
	}
	if len(current) > len(best) {
		best = current
	}
	return best
}
func dot(a, b []float64) float64 {
	v := 0.
	for i, x := range a {
		v += x * b[i]
	}
	return v
}
func mean(a []float64) float64 {
	if len(a) == 0 {
		return 0
	}
	v := 0.
	for _, x := range a {
		v += x
	}
	return v / float64(len(a))
}
func norm(a []float64) []float64 {
	n := math.Max(math.Sqrt(dot(a, a)), 1e-12)
	o := make([]float64, len(a))
	for i, v := range a {
		o[i] = v / n
	}
	return o
}
func z(a []float64) []float64 {
	m := mean(a)
	o := make([]float64, len(a))
	for i, v := range a {
		o[i] = v - m
	}
	s := math.Max(math.Sqrt(dot(o, o)/float64(len(o))), 1e-12)
	for i := range o {
		o[i] /= s
	}
	return o
}
func median(a []float64) float64 {
	v := append([]float64(nil), a...)
	sort.Float64s(v)
	i := len(v) / 2
	if len(v)%2 != 0 {
		return v[i]
	}
	return (v[i-1] + v[i]) / 2
}
func columns(a [][]float64) []float64 {
	o := make([]float64, len(a[0]))
	for _, r := range a {
		for i, v := range r {
			o[i] += v / float64(len(a))
		}
	}
	return o
}
func smooth(counts []float64) []float64 {
	total := 0.
	for i := range counts {
		counts[i] += 0.5
		total += counts[i]
	}
	for i := range counts {
		counts[i] = math.Sqrt(counts[i] / total)
	}
	return counts
}
func blocks(n []float64) [][]float64 {
	h := make([]float64, 355)
	for _, v := range n {
		h[int(v)-1]++
	}
	h = smooth(h)
	o := make([]float64, 0, 74)
	start := 0
	for i := 0; i < 4; i++ {
		size := len(n) / 4
		if i < len(n)%4 {
			size++
		}
		bins := make([]float64, 16)
		for _, v := range n[start : start+size] {
			bins[int((v-1)/355*16)]++
		}
		o = append(o, smooth(bins)...)
		start += size
	}
	last := make([]float64, 10)
	for _, v := range n {
		last[int(v)%10]++
	}
	o = append(o, smooth(last)...)
	return [][]float64{h, o}
}
func standard(a, m, s []float64) []float64 {
	o := make([]float64, len(a))
	for i, v := range a {
		o[i] = (v - m[i]) / s[i]
	}
	return o
}
func transform(b [][]float64, p []Params, weights []float64) []float64 {
	var o []float64
	for j, v := range b {
		v = norm(standard(v, p[j].Mean, p[j].Scale))
		for _, x := range v {
			o = append(o, x*math.Sqrt(weights[j]))
		}
	}
	return o
}
func subtract(a []float64, b [][]float64) []float64 {
	o := append([]float64(nil), a...)
	for _, row := range b {
		c := dot(a, row)
		for i, v := range row {
			o[i] -= c * v
		}
	}
	return o
}
func baseline(b [][]float64, h, o BaselinePart) []float64 {
	hu := norm(subtract(standard(b[0], h.Mean, h.Scale), h.Nuisance))
	m := make([]float64, len(h.Centroids))
	for i, r := range h.Centroids {
		m[i] = dot(hu, r)
	}
	oc := standard(b[1], o.Mean, o.Scale)
	raw := norm(oc)
	projected := norm(subtract(oc, o.Nuisance))
	t := make([]float64, len(m))
	n := make([]float64, len(m))
	for i, r := range o.Centroids {
		t[i] = math.Inf(-1)
		for _, env := range o.Environments {
			t[i] = math.Max(t[i], dot(raw, env[i]))
		}
		n[i] = dot(projected, r)
	}
	t = z(t)
	n = z(n)
	for i := range t {
		t[i] = 0.5*t[i] + 0.5*n[i]
	}
	t = z(t)
	m = z(m)
	for i := range m {
		m[i] = 0.5*m[i] + 0.5*t[i]
	}
	return m
}
func transitions(symbols []int, bins int) []float64 {
	joint := make([]float64, bins*bins)
	for i := range joint {
		joint[i] = 0.5
	}
	for i := 1; i < len(symbols); i++ {
		joint[symbols[i-1]*bins+symbols[i]]++
	}
	rows := make([]float64, bins)
	cols := make([]float64, bins)
	total := float64(len(symbols)-1) + 0.5*float64(bins*bins)
	for i := range joint {
		joint[i] /= total
		rows[i/bins] += joint[i]
		cols[i%bins] += joint[i]
	}
	for i := range joint {
		joint[i] -= rows[i/bins] * cols[i%bins]
	}
	return joint
}
func positional(n []float64) [][]float64 {
	b := blocks(n)
	kernel := make([]float64, 102)
	length := float64(len(n))
	for pos, v := range n {
		t := (float64(pos)+0.5)*2/length - 1
		basis := []float64{t, 0.5 * (3*t*t - 1), 0.5 * (5*t*t*t - 3*t)}
		for k := 0; k < 34; k++ {
			w := 0.
			if k < 24 {
				w = math.Exp(-0.5 * math.Pow((v-(1+float64(k)*354/23))/20, 2))
			} else if k-24 == int(v)%10 {
				w = 1
			}
			for j, a := range basis {
				kernel[k*3+j] += w * a / length
			}
		}
	}
	m := mean(n)
	variance := 0.
	for _, v := range n {
		variance += (v - m) * (v - m) / length
	}
	sd := math.Max(math.Sqrt(variance), 1)
	s := make([]float64, len(n))
	for i, v := range n {
		s[i] = (v - m) / sd
	}
	var seq []float64
	for _, lag := range []int{1, 2, 3, 5, 8, 13, 21, 34} {
		v := make([]float64, 6)
		count := len(n) - lag
		for i := 0; i < count; i++ {
			d := n[i+lag] - n[i]
			v[0] += s[i+lag] * s[i]
			v[1] += math.Abs(d) / 355
			if d > 0 {
				v[2]++
			}
			if d == 0 {
				v[3]++
			}
			if math.Abs(d) <= 5 {
				v[4]++
			}
			if math.Abs(d) <= 20 {
				v[5]++
			}
		}
		for _, x := range v {
			seq = append(seq, x/float64(count))
		}
	}
	sym := make([]int, len(n))
	for i, v := range n {
		sym[i] = int((v - 1) * 8 / 355)
	}
	seq = append(seq, transitions(sym, 8)...)
	for i, v := range n {
		sym[i] = int(v) % 10
	}
	seq = append(seq, transitions(sym, 10)...)
	extra := make([]float64, 5)
	for i := 0; i < len(n)-1; i++ {
		d := n[i+1] - n[i]
		if i < len(n)-2 {
			d2 := n[i+2] - n[i+1]
			if math.Copysign(1, d2) == math.Copysign(1, d) && ((d2 == 0) == (d == 0)) {
				extra[0]++
			}
			if d2 == d {
				extra[1]++
			}
		}
		if int(n[i])%10 == int(n[i+1])%10 {
			extra[2]++
		}
		step := math.Abs(s[i+1] - s[i])
		if step > 1 {
			extra[3]++
		}
		if step > 2 {
			extra[4]++
		}
	}
	for i, v := range extra {
		den := len(n) - 1
		if i < 2 {
			den--
		}
		seq = append(seq, v/float64(den))
	}
	for _, width := range []int{4, 8, 16} {
		sum := 0.
		for i := 0; i+width <= len(n); i++ {
			seen := map[int]bool{}
			for _, v := range n[i : i+width] {
				seen[int((v-1)*16/355)] = true
			}
			sum += float64(len(seen)) / float64(width)
		}
		seq = append(seq, sum/float64(len(n)-width+1))
	}
	return append(b, kernel, seq)
}
func (a *Artifact) rank(numbers [][]float64) []float64 {
	r := a.Ranker
	var full [][][]float64
	var ax, lda, base, pos [][]float64
	for _, n := range numbers {
		b := blocks(n)
		full = append(full, b)
		ax = append(ax, transform(b, r.Full, []float64{0.75, 0.25}))
		head := n
		if len(head) > 128 {
			head = head[:128]
		}
		x := transform(blocks(head), r.Head, []float64{0.75, 0.25})
		row := make([]float64, len(a.IDs))
		for i, w := range r.LDA {
			row[i] = dot(w, x) + r.Bias[i]
		}
		lda = append(lda, z(row))
		base = append(base, baseline(b, r.Bank.H, r.Bank.O))
		px := transform(positional(n), r.Positional.Params, []float64{0.6, 0.15, 0.125, 0.125})
		pr := make([]float64, len(a.IDs))
		for i, w := range r.Positional.Weights {
			pr[i] = dot(w, px) + r.Positional.Bias[i]
		}
		pos = append(pos, z(pr))
	}
	near := make([]float64, len(a.IDs))
	for i, refs := range r.References {
		v := make([]float64, len(ax))
		for j, x := range ax {
			d := make([]float64, len(refs))
			for k, ref := range refs {
				d[k] = math.Max(0, dot(x, x)+dot(ref, ref)-2*dot(x, ref))
			}
			sort.Float64s(d)
			count := len(d)
			if count > 7 {
				count = 7
			}
			v[j] = -mean(d[:count])
		}
		near[i] = median(v)
	}
	l := z(columns(lda))
	near = z(near)
	baseMean := z(columns(base))
	p := z(columns(pos))
	for i := range l {
		l[i] = 0.5*l[i] + 0.25*near[i] + 0.25*baseMean[i]
	}
	l = z(l)
	for i := range l {
		l[i] = r.Blend.Frequency*l[i] + r.Blend.Positional*p[i]
	}
	return l
}
func (a *Artifact) Analyze(outputs []Output, bank Bank) (result Analysis, err error) {
	// Malformed remote artifacts must fail closed, including dimension mismatches.
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("invalid reference detector dimensions")
		}
	}()
	result = Analysis{Decision: "unscorable", Results: []Candidate{}, Method: "shared-detector-v2", BankVersion: bank.BuiltAt, Reason: "insufficient_samples"}
	if a.Schema != "shared-detector-v2" || bank.BuiltAt != a.BuiltAt || len(bank.Models) != len(a.IDs) || len(a.IDs) < 2 || len(a.Counts) != len(a.IDs) {
		return result, fmt.Errorf("reference bank does not match detector")
	}
	for i, m := range bank.Models {
		if m.ID != a.IDs[i] || m.ResponseCount != a.Counts[i] {
			return result, fmt.Errorf("reference bank models do not match detector")
		}
	}
	var numbers [][]float64
	var ac, small []float64
	for _, o := range outputs {
		n := ParseNumbers(o.Text)
		if len(n) < int(math.Max(80, math.Ceil(float64(o.Expected)*0.55))) {
			continue
		}
		numbers = append(numbers, n)
		m := mean(n)
		variance, corr, steps := 0., 0., 0.
		for i, v := range n {
			variance += (v - m) * (v - m) / float64(len(n))
			if i > 0 {
				corr += (v - m) * (n[i-1] - m) / float64(len(n)-1)
				if math.Abs(v-n[i-1]) <= 20 {
					steps++
				}
			}
		}
		if variance == 0 {
			corr = 1
		} else {
			corr /= variance
		}
		ac = append(ac, corr)
		small = append(small, steps/float64(len(n)-1))
	}
	result.Used = len(numbers)
	if len(numbers) == 0 {
		return result, nil
	}
	if median(ac) > -0.23 && median(small) > 0.05 {
		result.Decision = "not_model"
		result.Reason = "sequence_shape"
		return result, nil
	}
	ranking := a.rank(numbers)
	var probabilities []float64
	c := a.Calibration
	if len(numbers) == 3 && c != nil && c.Schema == "shared-confidence-v2" && c.Method == "ranking-temperature" && c.Tau >= 0.001 && c.Tau <= 1000 && c.Binding.BaseHash == a.BaseHash && c.Binding.ReferenceHash == a.ReferenceHash && len(c.Binding.IDs) == len(a.IDs) {
		valid := true
		for i, id := range c.Binding.IDs {
			if id != a.IDs[i] {
				valid = false
			}
		}
		if valid {
			probabilities = make([]float64, len(ranking))
			maximum := math.Inf(-1)
			for _, v := range ranking {
				maximum = math.Max(maximum, v*c.Tau)
			}
			total := 0.
			for i, v := range ranking {
				probabilities[i] = math.Exp(v*c.Tau - maximum)
				total += probabilities[i]
			}
			for i := range probabilities {
				probabilities[i] /= total
			}
		}
	}
	for i, v := range ranking {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return result, fmt.Errorf("non-finite ranking")
		}
		candidate := Candidate{Model: a.IDs[i], DisplayName: bank.Models[i].DisplayName, Family: bank.Models[i].Family, Score: v}
		if probabilities != nil {
			p := probabilities[i]
			candidate.Probability = &p
		}
		result.Results = append(result.Results, candidate)
	}
	sort.SliceStable(result.Results, func(i, j int) bool { return result.Results[i].Score > result.Results[j].Score })
	result.Prediction = result.Results[0].Model
	result.Decision = "partial"
	result.Reason = "partial_samples"
	if len(numbers) == 3 {
		result.Decision = "scored"
		result.Reason = "reference_closed_set"
	}
	return result, nil
}

// Decode expands the byte-shuffled float32 detector container used by SDK 0.2.0.
func Decode(data []byte) (*Artifact, error) { return decode(data) }
