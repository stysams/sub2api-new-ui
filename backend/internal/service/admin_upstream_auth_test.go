package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type adminUpstreamTestEncryptor struct{}

func (adminUpstreamTestEncryptor) Encrypt(value string) (string, error) {
	return "encrypted:" + value, nil
}

func (adminUpstreamTestEncryptor) Decrypt(value string) (string, error) {
	if len(value) >= len("encrypted:") {
		return value[len("encrypted:"):], nil
	}
	return value, nil
}

func TestBuildUpstreamPrefersSuppliedSub2APITokens(t *testing.T) {
	service := &adminUpstreamService{
		encryptor: adminUpstreamTestEncryptor{},
		cfg:       &config.Config{},
	}

	upstream, err := service.buildUpstream(
		context.Background(),
		"apishop",
		0,
		"sub2api",
		"https://apishop.org",
		"access-token",
		"refresh-token",
		"admin@example.com",
		"",
		"should-not-be-used",
		"",
		nil,
		0,
	)

	require.NoError(t, err)
	require.Equal(t, "encrypted:access-token", upstream.TokenEncrypted)
	require.Equal(t, "encrypted:refresh-token", upstream.RefreshTokenEncrypted)
	require.Equal(t, "encrypted:should-not-be-used", upstream.PasswordEncrypted)
}

func TestAccessAndClientReusesStoredSub2APIAccessTokenBeforeReauthentication(t *testing.T) {
	service := &adminUpstreamService{
		encryptor: adminUpstreamTestEncryptor{},
		cfg:       &config.Config{},
	}
	expiresAt := time.Now().Add(-time.Hour)
	upstream := &Upstream{
		ID:                    7,
		Kind:                  "sub2api",
		BaseURL:               "https://apishop.org",
		TokenEncrypted:        "encrypted:stored-access-token",
		RefreshTokenEncrypted: "encrypted:stored-refresh-token",
		PasswordEncrypted:     "encrypted:stored-password",
		LoginIdentifier:       "admin@example.com",
		TokenExpiresAt:        &expiresAt,
	}

	access, _, err := service.accessAndClient(context.Background(), upstream)

	require.NoError(t, err)
	require.Equal(t, "stored-access-token", access.AccessToken)
}

func TestUpstreamEnvironmentProxyURLPrefersHTTPSProxy(t *testing.T) {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(key, "")
	}
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:10808")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:10809")
	require.Equal(t, "http://127.0.0.1:10809", upstreamEnvironmentProxyURL())
}
