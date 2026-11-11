package modelfingerprint

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"
)

func fixtureVector(n, seed int, scale float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.Sin(float64((i+1)*(seed+1))) * scale
	}
	return out
}
func fixtureMatrix(rows, cols, seed int) [][]float64 {
	out := make([][]float64, rows)
	for i := range out {
		out[i] = fixtureVector(cols, seed+i, 1)
	}
	return out
}
func fill(n int, value float64) []float64 {
	v := make([]float64, n)
	for i := range v {
		v[i] = value
	}
	return v
}
func fixtureParams(n int) Params { return Params{Mean: fill(n, 0.02), Scale: fill(n, 1)} }
func fixturePart(n int) BaselinePart {
	return BaselinePart{Mean: fill(n, 0.01), Scale: fill(n, 1), Nuisance: [][]float64{fixtureVector(n, 2, 0.05)}, Centroids: fixtureMatrix(3, n, 1), Environments: [][][]float64{fixtureMatrix(3, n, 3), fixtureMatrix(3, n, 4)}}
}
func fixture() (*Artifact, Bank, []Output) {
	a := &Artifact{Schema: "shared-detector-v2", BuiltAt: "fixture-v1", IDs: []string{"fixture-a", "fixture-b", "fixture-c"}, Counts: []int{50, 50, 50}, BaseHash: "base", ReferenceHash: "reference"}
	r := &a.Ranker
	r.Full = []Params{fixtureParams(355), fixtureParams(74)}
	r.Head = r.Full
	r.LDA = fixtureMatrix(3, 429, 4)
	r.Bias = []float64{0.1, 0.2, 0.3}
	for i := 0; i < 3; i++ {
		r.References = append(r.References, fixtureMatrix(8, 429, i+6))
	}
	r.Bank.H = fixturePart(355)
	r.Bank.O = fixturePart(74)
	r.Positional.Params = []Params{fixtureParams(355), fixtureParams(74), fixtureParams(102), fixtureParams(220)}
	r.Positional.Weights = fixtureMatrix(3, 751, 9)
	r.Positional.Bias = []float64{0.3, 0.2, 0.1}
	r.Blend.Frequency = 0.6
	r.Blend.Positional = 0.4
	raw := `{"schema":"shared-confidence-v2","method":"ranking-temperature","tau":1.3,"binding":{"base_sha256":"base","reference_sha256":"reference","model_ids":["fixture-a","fixture-b","fixture-c"]}}`
	_ = json.Unmarshal([]byte(raw), &a.Calibration)
	b := Bank{BuiltAt: a.BuiltAt}
	for _, id := range a.IDs {
		b.Models = append(b.Models, Model{ID: id, DisplayName: id, Family: "fixture", ResponseCount: 50})
	}
	var outputs []Output
	for j := 0; j < 3; j++ {
		var words []string
		for i := 0; i < 300; i++ {
			v := 1 + (i*17+j*13)%120
			if i%2 != 0 {
				v = 235 + (i*19+j*7)%120
			}
			words = append(words, strconv.Itoa(v))
		}
		outputs = append(outputs, Output{Text: strings.Join(words, ","), Expected: 300})
	}
	return a, b, outputs
}
func TestRankingMatchesReferenceSDK(t *testing.T) {
	a, b, outputs := fixture()
	result, err := a.Analyze(outputs, b)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != "scored" || result.Prediction != "fixture-c" {
		t.Fatalf("unexpected result: %+v", result)
	}
	// lmfpd-sdk 0.2.0, commit 1e58ed83, analyzeSharedOutputs, same deterministic fixture.
	scores := []float64{0.2874545019735204, -0.09472021692217636, -0.19273428505134427}
	probabilities := []float64{0.4663910161883036, 0.2837794177337712, 0.24982956607792534}
	for i, c := range result.Results {
		if math.Abs(c.Score-scores[i]) > 1e-9 {
			t.Fatalf("candidate %d score %.15f, want %.15f", i, c.Score, scores[i])
		}
		if c.Probability == nil || math.Abs(*c.Probability-probabilities[i]) > 1e-9 {
			t.Fatalf("candidate %d probability mismatch", i)
		}
	}
	result, err = a.Analyze(outputs[:1], b)
	if err != nil || result.Decision != "partial" || result.Results[0].Probability != nil {
		t.Fatalf("partial analysis: %+v %v", result, err)
	}
	a.Calibration.Binding.BaseHash = "invalid"
	result, err = a.Analyze(outputs, b)
	if err != nil || result.Results[0].Probability != nil {
		t.Fatal("mismatched calibration must not yield probabilities")
	}
}
func TestUnscorableAndSequenceGuard(t *testing.T) {
	a, b, _ := fixture()
	for _, tc := range []struct{ text, decision string }{{"hello", "unscorable"}, {strings.Repeat("17,", 300), "not_model"}} {
		r, err := a.Analyze([]Output{{Text: tc.text, Expected: 300}}, b)
		if err != nil || r.Decision != tc.decision || len(r.Results) != 0 {
			t.Fatalf("got %+v %v", r, err)
		}
	}
	b.BuiltAt = "different"
	if _, err := a.Analyze(nil, b); err == nil {
		t.Fatal("accepted mismatched bank")
	}
}
func TestParseLongestRun(t *testing.T) {
	got := ParseNumbers("example 1 and sequence 17, 203, 88, 0, 356 end 5")
	if len(got) != 3 || got[0] != 17 {
		t.Fatalf("%v", got)
	}
}
func TestDecodeRejectsMalformedData(t *testing.T) {
	for _, data := range [][]byte{nil, {1, 0, 0, 0}, {255, 255, 255, 255}} {
		if _, err := Decode(data); err == nil {
			t.Fatal("accepted malformed detector")
		}
	}
	header := []byte(`{"ranker":{"lda_bias":{"$f32":0,"n":1}}}`)
	start := (len(header) + 7) / 4 * 4
	raw := make([]byte, start+4)
	binary.LittleEndian.PutUint32(raw, uint32(len(header)))
	copy(raw[4:], header)
	binary.LittleEndian.PutUint32(raw[start:], math.Float32bits(1.25))
	a, err := Decode(raw)
	if err != nil || len(a.Ranker.Bias) != 1 || a.Ranker.Bias[0] != 1.25 {
		t.Fatalf("decode: %+v %v", a, err)
	}
}

func TestChallengeTemplates(t *testing.T) {
	out, err := GenerateChallenges()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatal("expected three challenges")
	}
	seen := map[int]bool{}
	for _, v := range out {
		if v.Expected < 292 || v.Expected > 332 || seen[v.Expected] || strings.Contains(v.Text, "__COUNT__") || !strings.Contains(v.Text, strconv.Itoa(v.Expected)) {
			t.Fatalf("invalid challenge %+v", v)
		}
		seen[v.Expected] = true
	}
}

func TestSnapshotChecksum(t *testing.T) {
	header := []byte(`{"schema":"shared-detector-v2","bank_built_at":"v","model_ids":["a","b"],"response_counts":[1,1]}`)
	data := make([]byte, (4+len(header)+3)/4*4)
	binary.LittleEndian.PutUint32(data, uint32(len(header)))
	copy(data[4:], header)
	bank := []byte(`{"schema":"lmfpd-data-v1","built_at":"v","models":[{"id":"a","response_count":1},{"id":"b","response_count":1}],"files":{"shared_detector.bin":{"sha256":"wrong"}}}`)
	if _, err := NewSnapshot(bank, data); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
}
