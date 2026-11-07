package securityaudit

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptRecordingPolicyRequiresCompleteKnownBooleanCategories(t *testing.T) {
	recommended := RecommendedPromptRecordingPolicy()
	require.NoError(t, ValidatePromptRecordingPolicy(recommended))
	require.Len(t, recommended.Keep, 16)
	for _, category := range []string{"user", "assistant", "tool_call", "tool_result"} {
		require.True(t, recommended.Keep[category])
	}
	for _, category := range []string{"thinking", "reasoning_summary", "tool_definition", "metadata"} {
		require.False(t, recommended.Keep[category])
	}
	t.Run("unsupported version", func(t *testing.T) {
		policy := ClonePromptRecordingPolicy(&recommended)
		policy.Version++
		require.ErrorContains(t, ValidatePromptRecordingPolicy(*policy), "version")
	})
	t.Run("missing category", func(t *testing.T) {
		policy := ClonePromptRecordingPolicy(&recommended)
		delete(policy.Keep, "thinking")
		require.ErrorContains(t, ValidatePromptRecordingPolicy(*policy), "thinking")
	})
	t.Run("unknown category", func(t *testing.T) {
		policy := ClonePromptRecordingPolicy(&recommended)
		policy.Keep["encrypted_content"] = true
		require.ErrorContains(t, ValidatePromptRecordingPolicy(*policy), "unknown category")
	})
	for _, raw := range []string{
		`{"version":1,"keep":{"user":null}}`,
		`{"version":1,"keep":{"user":"true"}}`,
		`{"version":1,"keep":{"user":1}}`,
		`{"version":1,"keep":{},"unexpected":true}`,
	} {
		var policy PromptRecordingPolicy
		require.Error(t, json.Unmarshal([]byte(raw), &policy), raw)
	}
}

func TestPromptRecordingPolicyDefaultsRecommendedAndPersists(t *testing.T) {
	ctx := context.Background()
	repository := &promptRecordingSettingRepository{}
	manager := NewConfigManager(nil, repository, nil, prefixEncryptor{}, testTotpKeyConfig())
	require.NoError(t, manager.Reload(ctx))
	require.Equal(t, RecommendedPromptRecordingPolicy(), *manager.PromptRecordingSnapshot().RecordingPolicy)
	policy := RecommendedPromptRecordingPolicy()
	policy.Keep["system"] = true
	require.NoError(t, manager.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{RecordingPolicy: &policy}))
	require.Contains(t, repository.values[SettingKeyPromptRecordingPolicy], `"system":true`)
	policy.Keep["system"] = false
	require.True(t, manager.PromptRecordingSnapshot().RecordingPolicy.Keep["system"], "save must detach the caller's map")
	snapshot := manager.PromptRecordingSnapshot()
	snapshot.RecordingPolicy.Keep["system"] = false
	require.True(t, manager.PromptRecordingSnapshot().RecordingPolicy.Keep["system"], "reads must detach the active map")
	disabled := false
	require.NoError(t, manager.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{HeadersEnabled: &disabled}))
	require.NoError(t, manager.Reload(ctx))
	require.False(t, manager.PromptRecordingSnapshot().HeadersEnabled)
	require.True(t, manager.PromptRecordingSnapshot().RecordingPolicy.Keep["system"], "scalar settings cannot replace the category policy")
	before := manager.PromptRecordingSnapshot()
	repository.writeError = errors.New("write failed")
	policy.Keep["metadata"] = true
	require.Error(t, manager.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{RecordingPolicy: &policy}))
	require.Equal(t, before, manager.PromptRecordingSnapshot())
}

type promptPolicySettingReadSpy struct {
	promptRecordingSettingRepository
	keys []string
}

func (r *promptPolicySettingReadSpy) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	r.keys = append([]string(nil), keys...)
	return r.promptRecordingSettingRepository.GetMultiple(ctx, keys)
}

func TestPromptRecordingIgnoresRetiredSettingsAndReturnsDetachedRecommendedPolicy(t *testing.T) {
	ctx := context.Background()
	retiredKeys := []string{
		"prompt_recording_filter_preset_enabled",
		"prompt_recording_filter_agent_preset_enabled",
		"prompt_recording_filter_skills_enabled",
	}
	repository := &promptPolicySettingReadSpy{}
	repository.values = map[string]string{}
	for _, key := range retiredKeys {
		repository.values[key] = "true"
	}
	manager := NewConfigManager(nil, repository, nil, prefixEncryptor{}, testTotpKeyConfig())
	require.NoError(t, manager.Reload(ctx))
	for _, key := range retiredKeys {
		require.NotContains(t, repository.keys, key, "retired settings must not be read from storage")
	}
	require.Equal(t, RecommendedPromptRecordingPolicy(), *manager.PromptRecordingSnapshot().RecordingPolicy)
	require.Equal(t, RecommendedPromptRecordingPolicy(), *(&PromptService{config: manager}).GetPromptRecordingConfig().RecordingPolicy)
	first := defaultPromptRecordingConfig()
	first.RecordingPolicy.Keep["user"] = false
	require.True(t, defaultPromptRecordingConfig().RecordingPolicy.Keep["user"])
	service := &PromptService{}
	require.Equal(t, RecommendedPromptRecordingPolicy(), *service.GetPromptRecordingConfig().RecordingPolicy)
}

func TestInvalidPromptRecordingPolicyPreservesSnapshotAndAuditActivation(t *testing.T) {
	ctx := context.Background()
	repository := &promptRecordingSettingRepository{}
	manager := NewConfigManager(nil, repository, nil, prefixEncryptor{}, testTotpKeyConfig())
	require.NoError(t, manager.Reload(ctx))
	policy := RecommendedPromptRecordingPolicy()
	require.NoError(t, manager.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{RecordingPolicy: &policy}))
	before := manager.PromptRecordingSnapshot()
	repository.values[SettingKeyPromptRecordingPolicy] = `{"version":999,"keep":{}}`
	repository.values[SettingKeyPromptRecordingHeaders] = "false"
	require.ErrorContains(t, manager.Reload(ctx), "invalid persisted prompt recording policy")
	require.Equal(t, before, manager.PromptRecordingSnapshot())
	_, active := manager.Active()
	require.True(t, active, "recording policy errors must not prevent audit config activation")
	disabled := false
	require.NoError(t, manager.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{HeadersEnabled: &disabled}))
	require.Equal(t, before.RecordingPolicy, manager.PromptRecordingSnapshot().RecordingPolicy, "a scalar update retains the earlier valid policy")

	startup := NewConfigManager(nil, repository, nil, prefixEncryptor{}, testTotpKeyConfig())
	require.Error(t, startup.Reload(ctx))
	require.False(t, startup.PromptRecordingEnabled(), "invalid initial policy must disable recording")
	enabled := true
	require.Error(t, startup.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{Enabled: &enabled}))
	require.NoError(t, startup.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{RecordingPolicy: &policy}))
	require.True(t, startup.PromptRecordingEnabled())
	require.False(t, startup.PromptRecordingSnapshot().HeadersEnabled)
}

func TestPromptRecordingSnapshotDoesNotMixPolicyAndScalarUpdates(t *testing.T) {
	ctx := context.Background()
	manager := NewConfigManager(nil, &promptRecordingSettingRepository{}, nil, prefixEncryptor{}, testTotpKeyConfig())
	policy := RecommendedPromptRecordingPolicy()
	flag := false
	require.NoError(t, manager.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{RecordingPolicy: &policy, HeadersEnabled: &flag}))
	var readers sync.WaitGroup
	stop := make(chan struct{})
	failures := make(chan struct{}, 1)
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
				config := manager.PromptRecordingSnapshot()
				if config.HeadersEnabled != config.RecordingPolicy.Keep["system"] {
					select {
					case failures <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
	for i := 0; i < 200; i++ {
		flag = !flag
		policy.Keep["system"] = flag
		require.NoError(t, manager.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{RecordingPolicy: &policy, HeadersEnabled: &flag}))
	}
	close(stop)
	readers.Wait()
	require.Empty(t, failures)
}

func TestInvalidInitialPromptPolicyPreservesScalarConfigAndRecoveryIntent(t *testing.T) {
	for _, rawPolicy := range []string{`{"version":`, `{"version":999,"keep":{}}`} {
		for _, enabled := range []bool{true, false} {
			t.Run(rawPolicy+"/enabled="+strconv.FormatBool(enabled), func(t *testing.T) {
				ctx := context.Background()
				values := map[string]string{
					SettingKeyPromptRecording:            strconv.FormatBool(enabled),
					SettingKeyPromptRecordingPolicy:      rawPolicy,
					SettingKeyPromptRecordingHeaders:     "false",
					SettingKeyPromptRecordingPrompt:      "false",
					SettingKeyPromptRecordingResponse:    "false",
					SettingKeyPromptRecordingRetention:   "17",
					SettingKeyPromptRecordingMaxMessages: "42",
				}
				// Both decoding and validation errors retain the parsed scalar
				// values so fail-closed startup can remember the enabled intent.
				parsed, err := parsePromptRecordingSettings(values)
				require.Error(t, err)
				require.Equal(t, enabled, parsed.Enabled)
				require.False(t, parsed.HeadersEnabled)
				require.False(t, parsed.PromptEnabled)
				require.False(t, parsed.ResponseEnabled)
				require.Equal(t, 17, parsed.RetentionDays)
				require.Equal(t, 42, parsed.MaxMessages)
				require.Equal(t, RecommendedPromptRecordingPolicy(), *parsed.RecordingPolicy)
				repository := &promptRecordingSettingRepository{staticSettingRepository: staticSettingRepository{values: values}}
				manager := NewConfigManager(nil, repository, nil, prefixEncryptor{}, testTotpKeyConfig())
				for range 2 {
					require.Error(t, manager.Reload(ctx))
					blocked := parsed
					blocked.Enabled = false
					require.Equal(t, blocked, manager.PromptRecordingSnapshot())
				}
				policy := RecommendedPromptRecordingPolicy()
				require.NoError(t, manager.SavePromptRecordingSettings(ctx, PromptRecordingSettingsUpdate{RecordingPolicy: &policy}))
				parsed.RecordingPolicy = &policy
				require.Equal(t, parsed, manager.PromptRecordingSnapshot(), "fixing only the policy restores persisted enabled intent and all scalar settings")
				require.NoError(t, manager.Reload(ctx))
				require.Equal(t, parsed, manager.PromptRecordingSnapshot())
			})
		}
	}
}
