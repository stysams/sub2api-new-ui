package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCreateUpstreamInputAcceptsSub2APIJSON(t *testing.T) {
	const configJSON = `{"name":"https://sub2api.stysams.online","kind":"sub2api","base_url":"https://sub2api.stysams.online","token":"","login_identifier":"123@qq.com","password":"example-password","notes":"","enabled":true}`

	var input CreateUpstreamInput
	require.NoError(t, json.Unmarshal([]byte(configJSON), &input))
	require.Equal(t, "https://sub2api.stysams.online", input.Name)
	require.Equal(t, domain.UpstreamKindSub2API, input.Kind)
	require.Empty(t, input.Token)
	require.Equal(t, "123@qq.com", input.LoginIdentifier)
	require.Equal(t, "example-password", input.Password)
	require.NotNil(t, input.Enabled)
	require.True(t, *input.Enabled)
}

func TestUpstreamURLAllowlistOnlyAppliesWhenEnabled(t *testing.T) {
	service := &adminUpstreamService{cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
		Enabled:           false,
		UpstreamHosts:     []string{"api.example.com"},
		AllowPrivateHosts: true,
	}}}}

	normalized, err := service.validateURL("https://sub2api.stysams.online/")
	require.NoError(t, err)
	require.Equal(t, "https://sub2api.stysams.online", normalized)

	service.cfg.Security.URLAllowlist.Enabled = true
	_, err = service.validateURL("https://sub2api.stysams.online")
	require.ErrorContains(t, err, "host is not allowed")
}

func TestSub2APIClientFetchesGroupsAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"data":[{"id":7,"name":"claude","rate_multiplier":1.25,"platform":"anthropic"}]}`))
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"mode":"quota_limited","quota":{"limit":1200,"used":300,"remaining":900,"unit":"USD"},"usage":{"total":{"requests":9}}}`))
		case "/api/v1/user/profile":
			_, _ = w.Write([]byte(`{"data":{"quota":1200,"used_quota":300,"request_count":9}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewUpstreamClient(domain.UpstreamKindSub2API, server.Client())
	require.NoError(t, err)
	access := &UpstreamAccess{BaseURL: server.URL, Kind: domain.UpstreamKindSub2API, AccessToken: "access-token"}

	groups, err := client.FetchGroups(context.Background(), access)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "7", groups[0].ID)
	require.Equal(t, "claude", groups[0].Name)
	require.Equal(t, 1.25, *groups[0].Ratio)

	usage, err := client.FetchAccountUsage(context.Background(), access)
	require.NoError(t, err)
	require.Equal(t, float64(1200), usage.Quota)
	require.Equal(t, float64(300), usage.UsedQuota)
	require.NotNil(t, usage.Remaining)
	require.Equal(t, float64(900), *usage.Remaining)
	require.Equal(t, int64(9), usage.RequestCount)
	require.Equal(t, "USD", usage.Currency)
}

func TestNewAPIClientUsesUserHeaderAndConvertsQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))
		require.Equal(t, "42", r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"success":true,"data":{"default":{"ratio":1,"desc":"default"}}}`))
		case "/v1/usage":
			http.NotFound(w, r)
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":2500000,"used_quota":500000,"request_count":7}}`))
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"display_in_currency":true,"quota_display_type":"USD","quota_per_unit":500000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewUpstreamClient(domain.UpstreamKindNewAPI, server.Client())
	require.NoError(t, err)
	access := &UpstreamAccess{BaseURL: server.URL, Kind: domain.UpstreamKindNewAPI, AccessToken: "access-token", UserID: "42"}

	groups, err := client.FetchGroups(context.Background(), access)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "default", groups[0].Name)
	require.Equal(t, float64(1), *groups[0].Ratio)

	usage, err := client.FetchAccountUsage(context.Background(), access)
	require.NoError(t, err)
	require.Equal(t, float64(5), usage.Quota)
	require.Equal(t, float64(1), usage.UsedQuota)
	require.NotNil(t, usage.Remaining)
	require.Equal(t, float64(5), *usage.Remaining)
	require.Equal(t, int64(7), usage.RequestCount)
	require.Equal(t, "USD", usage.Currency)
}

func TestLoginNewAPIReturnsAccessTokenAndUserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/login":
			require.Equal(t, http.MethodPost, r.Method)
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "user@example.com", body["username"])
			require.Equal(t, "example-password", body["password"])
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "session-value", Path: "/"})
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":42}}`))
		case "/api/user/token":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "42", r.Header.Get("New-Api-User"))
			cookie, err := r.Cookie("session")
			require.NoError(t, err)
			require.Equal(t, "session-value", cookie.Value)
			_, _ = w.Write([]byte(`{"success":true,"data":"access-token"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	login, err := loginNewAPI(context.Background(), server.URL, "user@example.com", "example-password", &config.Config{})
	require.NoError(t, err)
	require.Equal(t, "access-token", login.AccessToken)
	require.Equal(t, "42", login.UserID)
}

func TestLoginNewAPIAcceptsModernUserAndAccessTokenEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/login", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"access_token":"access-token","user":{"id":217}}}`))
	}))
	defer server.Close()

	login, err := loginNewAPI(context.Background(), server.URL, "user@example.com", "example-password", &config.Config{})
	require.NoError(t, err)
	require.Equal(t, "access-token", login.AccessToken)
	require.Equal(t, "217", login.UserID)
}

func TestSub2APIProfileTakesPrecedenceOverGatewayUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/user/profile":
			_, _ = w.Write([]byte(`{"data":{"quota":1200,"used_quota":300,"request_count":9}}`))
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"quota":{"limit":9999,"used":8888,"remaining":1111,"unit":"USD"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewUpstreamClient(domain.UpstreamKindSub2API, server.Client())
	require.NoError(t, err)
	usage, err := client.FetchAccountUsage(context.Background(), &UpstreamAccess{BaseURL: server.URL, Kind: domain.UpstreamKindSub2API, AccessToken: "access-token"})
	require.NoError(t, err)
	require.Equal(t, float64(1200), usage.Quota)
	require.Equal(t, float64(300), usage.UsedQuota)
	require.Equal(t, int64(9), usage.RequestCount)
	require.Equal(t, "USD", usage.Currency)
}

func TestSub2APIProfileBalancePopulatesQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/user/profile" {
			_, _ = w.Write([]byte(`{"data":{"balance":98.97}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := NewUpstreamClient(domain.UpstreamKindSub2API, server.Client())
	require.NoError(t, err)
	usage, err := client.FetchAccountUsage(context.Background(), &UpstreamAccess{BaseURL: server.URL, Kind: domain.UpstreamKindSub2API, AccessToken: "access-token"})
	require.NoError(t, err)
	require.Equal(t, 98.97, usage.Quota)
	require.NotNil(t, usage.Remaining)
	require.Equal(t, 98.97, *usage.Remaining)
}

func TestSub2APIProfileNestedUserBalancePopulatesQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/user/profile" {
			_, _ = w.Write([]byte(`{"data":{"user":{"balance":12.5,"request_count":3}}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := NewUpstreamClient(domain.UpstreamKindSub2API, server.Client())
	require.NoError(t, err)
	usage, err := client.FetchAccountUsage(context.Background(), &UpstreamAccess{BaseURL: server.URL, Kind: domain.UpstreamKindSub2API, AccessToken: "access-token"})
	require.NoError(t, err)
	require.Equal(t, 12.5, usage.Quota)
	require.Equal(t, int64(3), usage.RequestCount)
}

func TestModelWhitelistMappingUsesExactSelfMappings(t *testing.T) {
	require.Equal(t, map[string]string{"gpt-5.6": "gpt-5.6", "claude": "claude"}, modelWhitelistMapping([]string{"gpt-5.6", "", " claude "}))
}

func TestUsageSnapshotFromUsagePayloadWallet(t *testing.T) {
	snapshot := usageSnapshotFromUsagePayload(map[string]any{
		"mode":      "unrestricted",
		"balance":   42.5,
		"remaining": 42.5,
		"unit":      "USD",
		"usage": map[string]any{
			"total": map[string]any{
				"requests":    float64(17),
				"actual_cost": 3.25,
			},
		},
	})

	require.Equal(t, 42.5, snapshot.Quota)
	require.Equal(t, 3.25, snapshot.UsedQuota)
	require.Equal(t, int64(17), snapshot.RequestCount)
	require.Equal(t, "USD", snapshot.Currency)
}

func TestBalanceSnapshotKeepsZeroUsageFields(t *testing.T) {
	encoded, err := json.Marshal(domain.UpstreamBalanceSnapshot{})
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"quota":0`)
	require.Contains(t, string(encoded), `"used_quota":0`)
	require.Contains(t, string(encoded), `"request_count":0`)
}

func TestDoSub2LoginAcceptsDataEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/auth/login", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "123@qq.com", body["email"])
		require.Equal(t, "example-password", body["password"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"access_token":"access-token","refresh_token":"refresh-token","expires_in":86400}}`))
	}))
	defer server.Close()

	login, err := doSub2Login(context.Background(), server.Client(), server.URL, "/api/v1/auth/login", map[string]any{
		"email":    "123@qq.com",
		"password": "example-password",
	})
	require.NoError(t, err)
	require.Equal(t, "access-token", login.AccessToken)
	require.Equal(t, "refresh-token", login.RefreshToken)
	require.Equal(t, 86400, login.ExpiresIn)
	require.False(t, strings.Contains(login.AccessToken, "example-password"))
}
