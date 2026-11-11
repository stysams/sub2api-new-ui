package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	fp "github.com/Wei-Shaw/sub2api/internal/modelfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/tidwall/gjson"
)

type FingerprintSample struct {
	Prompt        string `json:"prompt"`
	ExpectedCount int    `json:"expected_count"`
	Count         int    `json:"count"`
	StatusCode    int    `json:"status_code"`
	ResponseModel string `json:"response_model,omitempty"`
	ResponseID    string `json:"response_id,omitempty"`
	Answer        string `json:"answer,omitempty"`
	Error         string `json:"error,omitempty"`
	LatencyMS     int64  `json:"latency_ms"`
}
type ModelFingerprintRecord struct {
	ID            int64               `json:"id"`
	AccountID     int64               `json:"account_id"`
	AccountName   string              `json:"account_name"`
	Platform      string              `json:"platform"`
	Model         string              `json:"model"`
	UpstreamModel string              `json:"upstream_model"`
	Endpoint      string              `json:"endpoint"`
	Status        string              `json:"status"`
	Score         int                 `json:"score"` // successful sample percentage, not identity confidence
	Samples       []FingerprintSample `json:"samples"`
	Analysis      *fp.Analysis        `json:"analysis"`
	CreatedAt     time.Time           `json:"created_at"`
}
type FingerprintFilter struct {
	Page      int
	PageSize  int
	AccountID int64
	Model     string
}
type ModelFingerprintRepository interface {
	Save(context.Context, *ModelFingerprintRecord) error
	List(context.Context, FingerprintFilter) ([]ModelFingerprintRecord, int64, error)
	LoadReference(context.Context) ([]byte, []byte, error)
	SaveReference(context.Context, []byte, []byte) error
}
type ModelFingerprintService struct {
	accounts AccountRepository
	records  ModelFingerprintRepository
	upstream HTTPUpstream
	cfg      *config.Config
	mu       sync.Mutex
	active   map[int64]bool
	bankMu   sync.Mutex
}

func NewModelFingerprintService(accounts AccountRepository, records ModelFingerprintRepository, upstream HTTPUpstream, cfg *config.Config) *ModelFingerprintService {
	return &ModelFingerprintService{accounts: accounts, records: records, upstream: upstream, cfg: cfg, active: map[int64]bool{}}
}

// Reference never downloads on page open. A run pins this exact snapshot.
func (s *ModelFingerprintService) Reference(ctx context.Context) (*fp.Snapshot, error) {
	b, d, err := s.records.LoadReference(ctx)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, errors.New("download fingerprint reference data first")
	}
	return fp.NewSnapshot(b, d)
}
func (s *ModelFingerprintService) RefreshReference(ctx context.Context) (*fp.Snapshot, error) {
	if !s.bankMu.TryLock() {
		return nil, errors.New("reference download already in progress")
	}
	defer s.bankMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	snapshot, err := fp.Download(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.records.SaveReference(ctx, snapshot.BankJSON, snapshot.Detector); err != nil {
		return nil, err
	}
	return snapshot, nil
}
func (s *ModelFingerprintService) Verify(ctx context.Context, accountID int64, model, api string) (*ModelFingerprintRecord, error) {
	model = strings.TrimSpace(model)
	if accountID <= 0 || model == "" || len(model) > 200 {
		return nil, errors.New("account_id and model are required (model max 200 characters)")
	}
	s.mu.Lock()
	if s.active[accountID] || len(s.active) >= 3 {
		s.mu.Unlock()
		return nil, errors.New("verification already running for this account or server is busy")
	}
	s.active[accountID] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.active, accountID); s.mu.Unlock() }()
	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("account not found")
	}
	if account.Type != AccountTypeAPIKey || account.ParentAccountID != nil || (!account.IsOpenAICompatible() && account.Platform != PlatformAnthropic) {
		return nil, errors.New("supported: direct OpenAI-compatible or Anthropic API key accounts; OAuth and inherited credentials are not supported")
	}
	if account.IsSyntheticUITest() {
		return nil, errors.New("synthetic accounts cannot be fingerprinted")
	}
	if api == "" || api == "auto" {
		api = "chat_completions"
		if account.Platform == PlatformAnthropic {
			api = "messages"
		} else if account.GetAPIProtocol() == APIProtocolAnthropic {
			api = "messages"
		} else if account.GetAPIProtocol() == APIProtocolResponses || (account.IsOpenAI() && openai_compat.ResolveResponsesSupport(account.Extra) == openai_compat.ResponsesSupportYes) {
			api = "responses"
		}
	}
	if api != "chat_completions" && api != "responses" && api != "messages" {
		return nil, errors.New("unsupported fingerprint API")
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, errors.New("account API key is missing")
	}
	base := strings.TrimSpace(account.GetCredential("base_url"))
	if account.IsMultiProtocolAPIKey() {
		protocol := APIProtocolChatCompletions
		if api == "responses" {
			protocol = APIProtocolResponses
		}
		if api == "messages" {
			protocol = APIProtocolAnthropic
		}
		if account.IsAdaptiveAPIProtocol() || base == "" {
			base = account.GetCNProtocolBaseURL(protocol)
		}
	}
	if base == "" {
		if account.Platform == PlatformOpenAI {
			base = "https://api.openai.com"
		} else if account.Platform == PlatformAnthropic {
			base = "https://api.anthropic.com"
		} else {
			return nil, errors.New("configure an upstream base_url for this account")
		}
	}
	validator := &AccountTestService{cfg: s.cfg}
	base, err = validator.validateUpstreamBaseURL(base)
	if err != nil {
		return nil, fmt.Errorf("upstream URL: %w", err)
	}
	u, err := url.Parse(base)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("upstream URL must not contain user info, query or fragment")
	}
	endpoint := buildOpenAIChatCompletionsURL(base)
	if api == "responses" {
		endpoint = buildOpenAIResponsesURL(base)
	} else if api == "messages" {
		endpoint = buildOpenAIEndpointURL(base, "/v1/messages")
	}
	snapshot, err := s.Reference(ctx)
	if err != nil {
		return nil, err
	}
	challenges, err := fp.GenerateChallenges()
	if err != nil {
		return nil, err
	}
	upstreamModel := account.GetMappedModel(model)
	evidence := func(reason string) *fp.Analysis {
		return &fp.Analysis{Decision: reason, Method: "shared-detector-v2", BankVersion: snapshot.Bank.BuiltAt, DetectorSHA256: snapshot.SHA256, Results: []fp.Candidate{}, Reason: reason}
	}
	record := &ModelFingerprintRecord{AccountID: accountID, AccountName: account.Name, Platform: account.Platform, Model: model, UpstreamModel: upstreamModel, Endpoint: endpoint, Status: "running", Samples: []FingerprintSample{}, Analysis: evidence("sampling")}
	if err = s.records.Save(ctx, record); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 360*time.Second)
	defer cancel()
	routeMismatch := false
	good := 0
	outputs := make([]fp.Output, 0, 3)
	for _, challenge := range challenges {
		sample := s.sample(ctx, account, endpoint, api, upstreamModel, token, challenge)
		// Redact secrets even when an upstream echoes credentials in response/error text.
		sample.Answer = strings.ReplaceAll(sample.Answer, token, "[redacted]")
		sample.Error = strings.ReplaceAll(sample.Error, token, "[redacted]")
		sample.ResponseModel = strings.ReplaceAll(sample.ResponseModel, token, "[redacted]")
		sample.ResponseID = strings.ReplaceAll(sample.ResponseID, token, "[redacted]")
		record.Samples = append(record.Samples, sample)
		progressCtx, progressCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		_ = s.records.Save(progressCtx, record)
		progressCancel()
		if sample.Error == "" {
			good++
			numbers := fp.ParseNumbers(sample.Answer)
			if len(numbers) > sample.ExpectedCount {
				numbers = numbers[:sample.ExpectedCount]
			}
			var values []string
			for _, n := range numbers {
				values = append(values, fmt.Sprintf("%.0f", n))
			}
			outputs = append(outputs, fp.Output{Text: strings.Join(values, ","), Expected: sample.ExpectedCount})
		}
		if sample.ResponseModel != "" && sample.ResponseModel != upstreamModel {
			routeMismatch = true
		}
	}
	record.Score = good * 100 / 3
	analysis, analysisErr := snapshot.Artifact.Analyze(outputs, snapshot.Bank)
	if analysisErr != nil {
		record.Status = "unscorable"
		record.Analysis = evidence("invalid_reference")
	} else {
		analysis.DetectorSHA256 = snapshot.SHA256
		record.Analysis = &analysis
		record.Status = analysis.Decision
	}
	if good == 0 {
		record.Status = "failed"
	}
	if routeMismatch {
		record.Status = "route_mismatch"
		record.Analysis = evidence("route_mismatch")
	}
	if ctx.Err() != nil {
		record.Status = "cancelled"
		record.Analysis = evidence("cancelled")
	}
	saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer saveCancel()
	if err = s.records.Save(saveCtx, record); err != nil {
		return nil, fmt.Errorf("save fingerprint record: %w", err)
	}
	return record, nil
}
func (s *ModelFingerprintService) sample(ctx context.Context, a *Account, endpoint, api, model, token string, c fp.Output) FingerprintSample {
	sample := FingerprintSample{Prompt: c.Text, ExpectedCount: c.Expected}
	started := time.Now()
	ctx, cancel := context.WithTimeout(WithHTTPUpstreamRedirectsDisabled(ctx), 110*time.Second)
	defer cancel()
	body := map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": c.Text}}, "max_tokens": 8192, "stream": false}
	if api == "responses" {
		body = map[string]any{"model": model, "input": []map[string]string{{"role": "user", "content": c.Text}}, "max_output_tokens": 8192, "store": false, "stream": false}
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(raw)))
	if err != nil {
		sample.Error = "unable to construct upstream request"
		return sample
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if api == "messages" {
		req.Header.Del("Authorization")
		setAnthropicAPIKeyAuthHeader(req.Header, a, token, a.GetCredential("base_url"))
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	a.ApplyHeaderOverrides(req.Header)
	resp, err := s.upstream.Do(req, resolveAccountProxyURL(a), a.ID, a.Concurrency)
	if err != nil {
		sample.Error = "upstream request failed"
		if ctx.Err() != nil {
			sample.Error = ctx.Err().Error()
		}
		sample.LatencyMS = time.Since(started).Milliseconds()
		return sample
	}
	raw, err = io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	_ = resp.Body.Close()
	sample.LatencyMS = time.Since(started).Milliseconds()
	sample.StatusCode = resp.StatusCode
	if err != nil || len(raw) > 1<<20 {
		sample.Error = "upstream response unreadable or too large"
		return sample
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		sample.Error = fmt.Sprintf("upstream HTTP %d", resp.StatusCode)
		return sample
	}
	if !gjson.ValidBytes(raw) {
		sample.Error = "invalid completion JSON"
		return sample
	}
	result := gjson.ParseBytes(raw)
	sample.ResponseModel = result.Get("model").String()
	sample.ResponseID = result.Get("id").String()
	switch api {
	case "responses":
		for _, item := range result.Get("output").Array() {
			if item.Get("type").String() == "message" {
				for _, text := range item.Get("content").Array() {
					if text.Get("type").String() == "output_text" {
						sample.Answer += text.Get("text").String()
					}
				}
			}
		}
		if result.Get("status").String() == "incomplete" {
			sample.Error = "truncated response"
		}
	case "messages":
		for _, text := range result.Get("content").Array() {
			if text.Get("type").String() == "text" {
				sample.Answer += text.Get("text").String()
			}
		}
		if result.Get("stop_reason").String() == "max_tokens" {
			sample.Error = "truncated response"
		}
	default:
		sample.Answer = result.Get("choices.0.message.content").String()
		if result.Get("choices.0.finish_reason").String() == "length" {
			sample.Error = "truncated response"
		}
	}
	sample.Count = len(fp.ParseNumbers(sample.Answer))
	if strings.TrimSpace(sample.Answer) == "" {
		sample.Error = "empty completion"
	}
	return sample
}
func (s *ModelFingerprintService) List(ctx context.Context, f FingerprintFilter) ([]ModelFingerprintRecord, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.records.List(ctx, f)
}
