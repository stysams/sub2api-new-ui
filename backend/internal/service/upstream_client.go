package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

const upstreamResponseLimit = 2 << 20

// UpstreamAccess is the short-lived, decrypted credential used only by the protocol layer.
type UpstreamAccess struct {
	BaseURL     string
	Kind        string
	AccessToken string
	UserID      string
}

type UpstreamLogin struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	UserID       string
}

type UpstreamCreatedKey struct {
	RemoteID string
	Name     string
	Secret   string
}

type UpstreamClient interface {
	FetchGroups(context.Context, *UpstreamAccess) ([]domain.UpstreamGroupItem, error)
	FetchAccountUsage(context.Context, *UpstreamAccess) (*domain.UpstreamBalanceSnapshot, error)
	CreateKey(context.Context, *UpstreamAccess, string, string) (*UpstreamCreatedKey, error)
	FetchKeySecret(context.Context, *UpstreamAccess, string) (string, error)
	FetchModels(context.Context, *UpstreamAccess, string) ([]string, error)
	ChatCompletion(context.Context, *UpstreamAccess, string, map[string]any) (*http.Response, error)
}

type upstreamClient struct {
	kind   string
	client *http.Client
}

// upstreamHTTPError keeps the response status available to the service layer.
// New API deployments commonly invalidate access tokens without changing the
// configured upstream, so callers can safely re-authenticate and retry only
// on an authentication response.
type upstreamHTTPError struct {
	StatusCode int
	Body       string
}

func (e *upstreamHTTPError) Error() string {
	return fmt.Sprintf("upstream returned HTTP %d: %s", e.StatusCode, e.Body)
}

func NewUpstreamClient(kind string, client *http.Client) (UpstreamClient, error) {
	if kind != domain.UpstreamKindNewAPI && kind != domain.UpstreamKindSub2API {
		return nil, fmt.Errorf("unsupported upstream kind: %s", kind)
	}
	if client == nil {
		return nil, fmt.Errorf("upstream HTTP client is required")
	}
	return &upstreamClient{kind: kind, client: client}, nil
}

func (c *upstreamClient) FetchGroups(ctx context.Context, access *UpstreamAccess) ([]domain.UpstreamGroupItem, error) {
	if c.kind == domain.UpstreamKindNewAPI {
		var payload struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Data    map[string]struct {
				Ratio any    `json:"ratio"`
				Desc  string `json:"desc"`
			} `json:"data"`
		}
		if err := c.getJSON(ctx, access, "/api/user/self/groups", &payload); err != nil {
			return nil, err
		}
		if !payload.Success && payload.Message != "" {
			return nil, fmt.Errorf("upstream groups: %s", payload.Message)
		}
		if !payload.Success {
			return nil, fmt.Errorf("upstream groups request failed")
		}
		items := make([]domain.UpstreamGroupItem, 0, len(payload.Data))
		for name, item := range payload.Data {
			items = append(items, domain.UpstreamGroupItem{Name: name, Ratio: numberPointer(item.Ratio), Desc: item.Desc})
		}
		return items, nil
	}

	var payload struct {
		Data []struct {
			ID             any     `json:"id"`
			Name           string  `json:"name"`
			RateMultiplier float64 `json:"rate_multiplier"`
			Platform       string  `json:"platform"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, access, "/api/v1/groups/available", &payload); err != nil {
		return nil, err
	}
	items := make([]domain.UpstreamGroupItem, 0, len(payload.Data))
	for _, item := range payload.Data {
		ratio := item.RateMultiplier
		items = append(items, domain.UpstreamGroupItem{ID: stringify(item.ID), Name: item.Name, Ratio: &ratio, Platform: item.Platform})
	}
	return items, nil
}

func (c *upstreamClient) FetchAccountUsage(ctx context.Context, access *UpstreamAccess) (*domain.UpstreamBalanceSnapshot, error) {
	var payload map[string]any
	path := "/api/user/self"
	if c.kind == domain.UpstreamKindSub2API {
		path = "/api/v1/user/profile"
	}
	profileErr := c.getJSON(ctx, access, path, &payload)
	if profileErr == nil {
		snapshot := c.accountUsageSnapshotFromProfile(ctx, access, payload)
		// Older Sub2API deployments only publish the display unit on /v1/usage.
		// Use that metadata without replacing the account-level figures above.
		if c.kind == domain.UpstreamKindSub2API && snapshot.Currency == "upstream quota points" {
			var usagePayload map[string]any
			if err := c.getJSON(ctx, access, "/v1/usage", &usagePayload); err == nil {
				if unit := usageSnapshotFromUsagePayload(usagePayload).Currency; unit != "" {
					snapshot.Currency = unit
				}
			}
		}
		return snapshot, nil
	}

	// Some older gateway-compatible deployments expose only /v1/usage.
	// Keep it as a compatibility fallback, but do not prefer key-level usage
	// over the account profile for Sub2API upstreams.
	var usagePayload map[string]any
	if err := c.getJSON(ctx, access, "/v1/usage", &usagePayload); err == nil {
		return usageSnapshotFromUsagePayload(usagePayload), nil
	}
	return nil, profileErr
}

func (c *upstreamClient) accountUsageSnapshotFromProfile(ctx context.Context, access *UpstreamAccess, payload map[string]any) *domain.UpstreamBalanceSnapshot {
	profile := payload
	if data, ok := payload["data"].(map[string]any); ok {
		profile = data
	}
	if c.kind == domain.UpstreamKindNewAPI {
		remaining := numberValue(profile["quota"])
		snapshot := &domain.UpstreamBalanceSnapshot{
			Quota:        remaining,
			UsedQuota:    numberValue(firstValue(profile, "used_quota", "usedQuota", "usage")),
			Remaining:    &remaining,
			RequestCount: int64(numberValue(firstValue(profile, "request_count", "requestCount"))),
			Currency:     "upstream quota points",
			FetchedAt:    time.Now().UTC().Format(time.RFC3339),
		}
		var statusPayload map[string]any
		if err := c.getJSON(ctx, access, "/api/status", &statusPayload); err == nil {
			if data, ok := statusPayload["data"].(map[string]any); ok {
				applyNewAPIQuotaDisplay(snapshot, data)
			}
		}
		return snapshot
	}
	quota := numberValue(profileValue(profile, "quota", "total_quota", "quota_limit"))
	used := numberValue(profileValue(profile, "used_quota", "usedQuota", "usage"))
	remaining := numberPointer(profileValue(profile, "remaining", "balance", "remaining_quota", "remain_quota"))
	// Sub2API's user profile exposes the account wallet as `balance` rather
	// than `quota`. Treat it as the total quota as well as the remaining value
	// when an explicit quota field is absent, otherwise the UI shows quota=0.
	if quota == 0 {
		if balance := numberPointer(profileValue(profile, "balance", "remaining_quota", "remain_quota")); balance != nil {
			quota = *balance
		}
	}
	if remaining == nil && quota > 0 && used >= 0 && quota >= used {
		value := quota - used
		remaining = &value
	}
	snapshot := &domain.UpstreamBalanceSnapshot{
		Quota:        quota,
		UsedQuota:    used,
		Remaining:    remaining,
		RequestCount: int64(numberValue(profileValue(profile, "request_count", "requestCount", "requests"))),
		Currency:     "upstream quota points",
		FetchedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	return snapshot
}

func applyNewAPIQuotaDisplay(snapshot *domain.UpstreamBalanceSnapshot, status map[string]any) {
	if snapshot == nil || !boolValue(status["display_in_currency"]) {
		return
	}
	quotaPerUnit := numberValue(status["quota_per_unit"])
	if quotaPerUnit <= 0 {
		return
	}
	factor := 1 / quotaPerUnit
	currency := "USD"
	switch strings.ToUpper(stringify(status["quota_display_type"])) {
	case "CNY":
		factor *= numberValue(status["usd_exchange_rate"])
		currency = "CNY"
	case "CUSTOM":
		factor *= numberValue(status["custom_currency_exchange_rate"])
		currency = stringify(status["custom_currency_symbol"])
		if currency == "" {
			currency = "CUSTOM"
		}
	}
	snapshot.Quota *= factor
	snapshot.UsedQuota *= factor
	if snapshot.Remaining != nil {
		remaining := *snapshot.Remaining * factor
		snapshot.Remaining = &remaining
	}
	snapshot.Currency = currency
}

func usageSnapshotFromUsagePayload(payload map[string]any) *domain.UpstreamBalanceSnapshot {
	if data, ok := payload["data"].(map[string]any); ok {
		payload = data
	}

	var quota, used, requests float64
	var remaining *float64
	var quotaUnit any
	if quotaObject, ok := payload["quota"].(map[string]any); ok {
		quota = numberValue(firstValue(quotaObject, "limit", "quota", "remaining"))
		used = numberValue(firstValue(quotaObject, "used", "used_quota", "usedQuota"))
		remaining = numberPointer(firstValue(quotaObject, "remaining"))
		quotaUnit = firstValue(quotaObject, "unit", "currency")
	} else {
		quota = numberValue(firstValue(payload, "quota", "balance", "remaining"))
		used = numberValue(firstValue(payload, "used_quota", "usedQuota", "used"))
		remaining = numberPointer(firstValue(payload, "remaining"))
	}
	if balanceObject, ok := payload["balance"].(map[string]any); ok && quota == 0 {
		balanceValue := firstValue(balanceObject, "amount", "value", "balance", "remaining")
		quota = numberValue(balanceValue)
		if remaining == nil {
			remaining = numberPointer(firstValue(balanceObject, "remaining", "amount", "value", "balance"))
		}
		if quotaUnit == nil {
			quotaUnit = firstValue(balanceObject, "unit", "currency")
		}
	}
	if usageObject, ok := payload["usage"].(map[string]any); ok {
		if totalObject, ok := usageObject["total"].(map[string]any); ok {
			requests = numberValue(firstValue(totalObject, "requests", "request_count", "requestCount"))
			if used == 0 {
				used = numberValue(firstValue(totalObject, "actual_cost", "cost", "used_quota", "usedQuota"))
			}
		}
		if requests == 0 {
			requests = numberValue(firstValue(usageObject, "requests", "request_count", "requestCount"))
		}
	}
	if requests == 0 {
		requests = numberValue(firstValue(payload, "request_count", "requestCount", "requests"))
	}

	currency := stringify(firstValue(payload, "unit", "currency"))
	if currency == "" {
		currency = stringify(quotaUnit)
	}
	if currency == "" {
		currency = "upstream quota points"
	}
	return &domain.UpstreamBalanceSnapshot{
		Quota:        quota,
		UsedQuota:    used,
		Remaining:    remaining,
		RequestCount: int64(requests),
		Currency:     currency,
		FetchedAt:    time.Now().UTC().Format(time.RFC3339),
	}
}

func (c *upstreamClient) CreateKey(ctx context.Context, access *UpstreamAccess, group, name string) (*UpstreamCreatedKey, error) {
	if c.kind == domain.UpstreamKindNewAPI {
		body := map[string]any{
			"name": name, "group": group, "unlimited_quota": true, "expired_time": -1,
			"remain_quota": 0, "model_limits_enabled": false, "model_limits": "",
		}
		var created struct {
			Success bool           `json:"success"`
			Message string         `json:"message"`
			Data    map[string]any `json:"data"`
		}
		if err := c.postJSON(ctx, access, "/api/token/", body, &created); err != nil {
			return nil, err
		}
		if !created.Success && created.Message != "" {
			return nil, fmt.Errorf("create upstream token: %s", created.Message)
		}
		remoteID := stringify(created.Data["id"])
		if remoteID == "" {
			var list struct {
				Data struct {
					Items []map[string]any `json:"items"`
				} `json:"data"`
			}
			if err := c.getJSON(ctx, access, "/api/token/", &list); err != nil {
				return nil, err
			}
			for i := len(list.Data.Items) - 1; i >= 0; i-- {
				if stringify(list.Data.Items[i]["name"]) == name {
					remoteID = stringify(list.Data.Items[i]["id"])
					break
				}
			}
		}
		if remoteID == "" {
			return nil, fmt.Errorf("created upstream token ID was not returned")
		}
		secret, err := c.FetchKeySecret(ctx, access, remoteID)
		if err != nil {
			return nil, err
		}
		return &UpstreamCreatedKey{RemoteID: remoteID, Name: name, Secret: secret}, nil
	}

	groupID, err := strconv.ParseInt(group, 10, 64)
	if err != nil || groupID <= 0 {
		return nil, fmt.Errorf("invalid sub2api group id: %s", group)
	}
	var created struct {
		Data struct {
			ID   any    `json:"id"`
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"data"`
	}
	if err := c.postJSON(ctx, access, "/api/v1/keys", map[string]any{"name": name, "group_id": groupID}, &created); err != nil {
		return nil, err
	}
	return &UpstreamCreatedKey{RemoteID: stringify(created.Data.ID), Name: created.Data.Name, Secret: created.Data.Key}, nil
}

func (c *upstreamClient) FetchKeySecret(ctx context.Context, access *UpstreamAccess, remoteID string) (string, error) {
	if c.kind == domain.UpstreamKindSub2API {
		return "", fmt.Errorf("sub2api key secret is returned during creation")
	}
	var payload struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := c.postJSON(ctx, access, "/api/token/"+remoteID+"/key", nil, &payload); err != nil {
		return "", err
	}
	if payload.Data.Key == "" {
		return "", fmt.Errorf("upstream token secret is empty")
	}
	return payload.Data.Key, nil
}

func (c *upstreamClient) FetchModels(ctx context.Context, access *UpstreamAccess, secret string) ([]string, error) {
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.requestJSON(ctx, http.MethodGet, access, "/v1/models", secret, nil, &payload); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) != "" {
			models = append(models, item.ID)
		}
	}
	return models, nil
}

func (c *upstreamClient) ChatCompletion(ctx context.Context, access *UpstreamAccess, secret string, body map[string]any) (*http.Response, error) {
	return c.rawRequest(ctx, http.MethodPost, access, "/v1/chat/completions", secret, body)
}

func (c *upstreamClient) getJSON(ctx context.Context, access *UpstreamAccess, path string, out any) error {
	return c.requestJSON(ctx, http.MethodGet, access, path, access.AccessToken, nil, out)
}

func (c *upstreamClient) postJSON(ctx context.Context, access *UpstreamAccess, path string, body any, out any) error {
	return c.requestJSON(ctx, http.MethodPost, access, path, access.AccessToken, body, out)
}

func (c *upstreamClient) requestJSON(ctx context.Context, method string, access *UpstreamAccess, path, token string, body any, out any) error {
	resp, err := c.rawRequest(ctx, method, access, path, token, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, upstreamResponseLimit))
	if err != nil {
		return fmt.Errorf("read upstream response: %w", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode upstream response: %w", err)
	}
	return nil
}

func (c *upstreamClient) rawRequest(ctx context.Context, method string, access *UpstreamAccess, path, token string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode upstream request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	url := strings.TrimRight(access.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if access.Kind == domain.UpstreamKindNewAPI && access.UserID != "" && token == access.AccessToken {
		req.Header.Set("New-Api-User", access.UserID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request upstream: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, upstreamResponseLimit))
		resp.Body.Close()
		return nil, &upstreamHTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	return resp, nil
}

func numberPointer(value any) *float64 {
	n, ok := number(value)
	if !ok {
		return nil
	}
	return &n
}
func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(v)
		return err == nil && parsed
	default:
		return false
	}
}
func numberValue(value any) float64 { n, _ := number(value); return n }
func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	default:
		return 0, false
	}
}
func stringify(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
func firstValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func profileValue(profile map[string]any, keys ...string) any {
	if value := firstValue(profile, keys...); value != nil {
		return value
	}
	for _, nestedKey := range []string{"user", "account", "profile", "wallet"} {
		if nested, ok := profile[nestedKey].(map[string]any); ok {
			if value := firstValue(nested, keys...); value != nil {
				return value
			}
		}
	}
	return nil
}

func newUpstreamHTTPClient(cfgValidate, allowPrivate bool) (*http.Client, error) {
	return httpclient.GetClient(httpclient.Options{Timeout: 20 * time.Second, ResponseHeaderTimeout: 15 * time.Second, ValidateResolvedIP: cfgValidate, AllowPrivateHosts: allowPrivate})
}
