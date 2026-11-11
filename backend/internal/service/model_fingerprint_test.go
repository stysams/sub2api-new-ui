package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	fp "github.com/Wei-Shaw/sub2api/internal/modelfingerprint"
)

type fingerprintAccounts struct {
	AccountRepository
	account *Account
}

func (r *fingerprintAccounts) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

type fingerprintRecords struct {
	items          []ModelFingerprintRecord
	bank, detector []byte
}

func (r *fingerprintRecords) Save(_ context.Context, v *ModelFingerprintRecord) error {
	if v.ID == 0 {
		v.ID = 1
		v.CreatedAt = time.Now()
	}
	clone := *v
	clone.Samples = append([]FingerprintSample(nil), v.Samples...)
	r.items = append(r.items, clone)
	return nil
}
func (r *fingerprintRecords) List(context.Context, FingerprintFilter) ([]ModelFingerprintRecord, int64, error) {
	return r.items, int64(len(r.items)), nil
}
func (r *fingerprintRecords) LoadReference(context.Context) ([]byte, []byte, error) {
	return r.bank, r.detector, nil
}
func (r *fingerprintRecords) SaveReference(_ context.Context, b, d []byte) error {
	r.bank = b
	r.detector = d
	return nil
}

type fingerprintUpstream struct {
	HTTPUpstream
	calls int
	do    func(*http.Request) (*http.Response, error)
}

func (u *fingerprintUpstream) Do(req *http.Request, _ string, id int64, _ int) (*http.Response, error) {
	if id != 7 {
		panic("account was not pinned")
	}
	u.calls++
	return u.do(req)
}
func fingerprintTestReference(t *testing.T, r *fingerprintRecords) {
	t.Helper()
	header := []byte(`{"schema":"shared-detector-v2","bank_built_at":"test-version","model_ids":["a","b"],"response_counts":[1,1]}`)
	r.detector = make([]byte, (4+len(header)+3)/4*4)
	binary.LittleEndian.PutUint32(r.detector, uint32(len(header)))
	copy(r.detector[4:], header)
	sum := sha256.Sum256(r.detector)
	r.bank = []byte(`{"schema":"lmfpd-data-v1","built_at":"test-version","models":[{"id":"a","response_count":1},{"id":"b","response_count":1}],"files":{"shared_detector.bin":{"sha256":"` + hex.EncodeToString(sum[:]) + `"}}}`)
}
func newFingerprintFixture(t *testing.T) (*ModelFingerprintService, *fingerprintRecords, *fingerprintUpstream) {
	r := &fingerprintRecords{}
	fingerprintTestReference(t, r)
	a := &fingerprintAccounts{account: &Account{ID: 7, Name: "test", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "secret-token", "base_url": "https://upstream.example"}}}
	u := &fingerprintUpstream{}
	return NewModelFingerprintService(a, r, u, &config.Config{}), r, u
}
func TestModelFingerprintPinsMappingAndDoesNotTrustEcho(t *testing.T) {
	svc, records, u := newFingerprintFixture(t)
	svc.accounts.(*fingerprintAccounts).account.Credentials["model_mapping"] = map[string]any{"alias": "actual"}
	u.do = func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://upstream.example/v1/chat/completions" || req.Header.Get("Authorization") != "Bearer secret-token" || !HTTPUpstreamRedirectsDisabled(req.Context()) {
			t.Fatalf("unexpected request %s", req.URL)
		}
		body, _ := io.ReadAll(req.Body)
		var v map[string]any
		_ = json.Unmarshal(body, &v)
		if v["model"] != "actual" || v["max_tokens"] != float64(8192) {
			t.Fatalf("bad payload %s", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"model":"actual","choices":[{"message":{"content":"hello secret-token"}}]}`))}, nil
	}
	result, err := svc.Verify(context.Background(), 7, "alias", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if u.calls != 3 || result.Status != "unscorable" || result.Analysis == nil || result.Analysis.Used != 0 {
		t.Fatalf("ordinary successful completions cannot prove identity: %+v", result)
	}
	if records.items[0].Status != "running" || result.ID != 1 {
		t.Fatal("task should be persisted before requests")
	}
	if strings.Contains(result.Samples[0].Answer, "secret-token") {
		t.Fatal("credential leaked")
	}
}
func TestModelFingerprintProtocolsAndMismatch(t *testing.T) {
	for _, api := range []string{"responses", "messages"} {
		t.Run(api, func(t *testing.T) {
			svc, _, u := newFingerprintFixture(t)
			u.do = func(req *http.Request) (*http.Response, error) {
				want := "/v1/responses"
				body := `{"model":"another-model","output":[{"type":"message","content":[{"type":"output_text","text":"17, 203"}]}]}`
				if api == "messages" {
					want = "/v1/messages"
					body = `{"model":"another-model","content":[{"type":"text","text":"17, 203"}]}`
					if req.Header.Get("x-api-key") != "secret-token" || req.Header.Get("Authorization") != "" {
						t.Fatal("bad messages auth")
					}
				}
				if req.URL.Path != want {
					t.Fatal(req.URL.Path)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			}
			result, err := svc.Verify(context.Background(), 7, "model", api)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "route_mismatch" || result.Analysis == nil || len(result.Analysis.Results) != 0 || result.Analysis.BankVersion != "test-version" {
				t.Fatal("route mismatch was scored")
			}
		})
	}
}
func TestModelFingerprintFailedRequestsAreRecorded(t *testing.T) {
	svc, r, u := newFingerprintFixture(t)
	u.do = func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(`secret-token`))}, nil
	}
	result, err := svc.Verify(context.Background(), 7, "model", "auto")
	if err != nil || result.Status != "failed" || len(r.items) < 2 {
		t.Fatalf("failed test not saved: %+v %v", result, err)
	}
}
func TestModelFingerprintValidationAndNoImplicitDownload(t *testing.T) {
	svc, r, u := newFingerprintFixture(t)
	r.bank = nil
	if _, err := svc.Verify(context.Background(), 7, "model", "auto"); err == nil || u.calls != 0 {
		t.Fatal("missing reference should not make upstream calls")
	}
	for _, kind := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		svc.accounts.(*fingerprintAccounts).account.Type = kind
		if _, err := svc.Verify(context.Background(), 7, "model", "auto"); err == nil {
			t.Fatal("unsupported account accepted")
		}
	}
	if _, err := fp.Decode(nil); err == nil {
		t.Fatal("invalid detector accepted")
	}
}

func TestModelFingerprintCancellationPersistsReceivedEvidence(t *testing.T) {
	svc, r, u := newFingerprintFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	u.do = func(*http.Request) (*http.Response, error) {
		cancel()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"model":"model","choices":[{"message":{"content":"17, 203"}}]}`))}, nil
	}
	result, err := svc.Verify(ctx, 7, "model", "auto")
	if err != nil || result.Status != "cancelled" || result.Analysis == nil || len(result.Analysis.Results) != 0 || len(r.items) < 2 {
		t.Fatalf("cancellation not saved: %+v %v", result, err)
	}
}
