package securityaudit

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptRecordingPolicySettingsAPI(t *testing.T) {
	service := &fakePromptAdminService{recording: defaultPromptRecordingConfig()}
	router := promptAdminRouter(service)
	policy := RecommendedPromptRecordingPolicy()
	updated := promptAdminRequest(t, router, http.MethodPut, "/admin/prompt-records/recording", map[string]any{"recording_policy": policy})
	require.Equal(t, http.StatusOK, updated.Code)
	require.Equal(t, &policy, service.recording.RecordingPolicy)
	before := clonePromptRecordingConfig(service.recording)
	for _, body := range []map[string]any{
		{"filter_preset": true},
		{"filter_agent_preset": false},
		{"filter_skills": false},
		{"enabled": true, "filter_preset": false},
		{"enabled": true, "filter_agent_preset": true},
		{"enabled": true, "filter_skills": true},
		{"recording_policy": policy, "filter_preset": true},
		{"recording_policy": policy, "filter_agent_preset": false},
		{"recording_policy": policy, "filter_skills": false},
		{"recording_policy": map[string]any{"version": 1, "keep": map[string]bool{"user": true}}},
		{"recording_policy": map[string]any{"version": 2, "keep": policy.Keep}},
		{"enabled": true, "unrecognized_setting": true},
	} {
		result := promptAdminRequest(t, router, http.MethodPut, "/admin/prompt-records/recording", body)
		require.Equal(t, http.StatusBadRequest, result.Code, result.Body.String())
		require.Equal(t, before, service.recording)
	}
	get := promptAdminRequest(t, router, http.MethodGet, "/admin/prompt-records/recording", nil)
	require.Equal(t, http.StatusOK, get.Code)
	for _, removed := range []string{"filter_preset", "filter_agent_preset", "filter_skills"} {
		require.NotContains(t, get.Body.String(), removed)
	}
	require.Contains(t, get.Body.String(), "recording_policy")
}

func TestPromptRecordingPreviewUsesCandidatePolicyWithoutSaving(t *testing.T) {
	service := &fakePromptAdminService{recording: defaultPromptRecordingConfig()}
	router := promptAdminRouter(service)
	policy := RecommendedPromptRecordingPolicy()
	before := clonePromptRecordingConfig(service.recording)
	body := map[string]any{
		"protocol": "openai_chat_completions",
		"body": map[string]any{
			"messages": []any{
				map[string]any{"role": "system", "content": "SYSTEM_PREVIEW_CANARY"},
				map[string]any{"role": "user", "content": "USER_PREVIEW_CANARY"},
				map[string]any{"role": "assistant", "content": "ASSISTANT_PREVIEW_CANARY", "reasoning_content": "THINKING_PREVIEW_CANARY"},
			},
			"encrypted_content": "ENCRYPTED_PREVIEW_CANARY",
		},
		"recording_policy": policy,
	}
	result := promptAdminRequest(t, router, http.MethodPost, "/admin/prompt-records/preview", body)
	require.Equal(t, http.StatusOK, result.Code, result.Body.String())
	require.Contains(t, result.Body.String(), "content_document")
	require.Contains(t, result.Body.String(), "message_count")
	require.Contains(t, result.Body.String(), "USER_PREVIEW_CANARY")
	require.Contains(t, result.Body.String(), "ASSISTANT_PREVIEW_CANARY")
	require.NotContains(t, result.Body.String(), "SYSTEM_PREVIEW_CANARY")
	require.NotContains(t, result.Body.String(), "THINKING_PREVIEW_CANARY")
	require.NotContains(t, result.Body.String(), "ENCRYPTED_PREVIEW_CANARY")
	require.Equal(t, before, service.recording)
	policy.Keep["system"] = true
	body["recording_policy"] = policy
	result = promptAdminRequest(t, router, http.MethodPost, "/admin/prompt-records/preview", body)
	require.Equal(t, http.StatusOK, result.Code)
	require.Contains(t, result.Body.String(), "SYSTEM_PREVIEW_CANARY")
	require.Equal(t, before, service.recording)
}

func TestPromptRecordingPreviewRejectsInvalidAndOversizeRequests(t *testing.T) {
	router := promptAdminRouter(&fakePromptAdminService{})
	policy := RecommendedPromptRecordingPolicy()
	for _, body := range []map[string]any{
		{"protocol": "responses", "body": map[string]any{}},
		{"body": map[string]any{}, "recording_policy": policy},
		{"protocol": "responses", "recording_policy": policy},
		{"protocol": "responses", "body": nil, "recording_policy": policy},
		{"protocol": "responses", "body": map[string]any{}, "recording_policy": policy, "max_messages": 0},
		{"protocol": "responses", "body": map[string]any{}, "recording_policy": policy, "max_messages": 1000},
	} {
		result := promptAdminRequest(t, router, http.MethodPost, "/admin/prompt-records/preview", body)
		require.Equal(t, http.StatusBadRequest, result.Code, result.Body.String())
	}
	result := promptAdminRequest(t, router, http.MethodPost, "/admin/prompt-records/preview", map[string]any{
		"protocol": "responses", "body": strings.Repeat("x", 1<<20), "recording_policy": policy,
	})
	require.Equal(t, http.StatusRequestEntityTooLarge, result.Code)
}
