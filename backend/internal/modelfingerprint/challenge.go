package modelfingerprint

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Exact multilingual templates from lmfpd-sdk 0.2.0, MIT (see LICENSE).
//
//go:embed challenge_templates.json
var challengeTemplates []byte

type challengeLanguage struct {
	Space      string   `json:"space"`
	Openings   []string `json:"openings"`
	Actions    []string `json:"actions"`
	Request    string   `json:"request"`
	Rules      string   `json:"rules"`
	Endings    []string `json:"endings"`
	Separators []string `json:"separators"`
	Start      string   `json:"start"`
}

func chooseIndex(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}
func GenerateChallenges() ([]Output, error) {
	var langs []challengeLanguage
	if err := json.Unmarshal(challengeTemplates, &langs); err != nil {
		return nil, err
	}
	used := map[int]bool{}
	outputs := make([]Output, 3)
	for i := range outputs {
		count := 0
		for {
			v, err := chooseIndex(41)
			if err != nil {
				return nil, err
			}
			count = 292 + v
			if !used[count] {
				used[count] = true
				break
			}
		}
		index, err := chooseIndex(len(langs))
		if err != nil {
			return nil, err
		}
		lang := langs[index]
		choices := make([]string, 4)
		for j, options := range [][]string{lang.Openings, lang.Actions, lang.Endings, lang.Separators} {
			if len(options) == 0 {
				return nil, fmt.Errorf("missing challenge templates")
			}
			v, err := chooseIndex(len(options))
			if err != nil {
				return nil, err
			}
			choices[j] = options[v]
		}
		request := strings.NewReplacer("__ACTION__", choices[1], "__COUNT__", strconv.Itoa(count)).Replace(lang.Request)
		outputs[i] = Output{Expected: count, Text: strings.Join([]string{choices[0], request, lang.Rules, choices[2], choices[3], lang.Start}, lang.Space)}
	}
	return outputs, nil
}
