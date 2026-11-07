package securityaudit

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const SettingKeyPromptRecordingPolicy = "prompt_recording_policy"

const promptRecordingPolicyVersion = 1

var promptRecordingPolicyCategories = []string{
	"user", "assistant", "tool_call", "tool_result", "system", "developer", "instructions",
	"project", "agent", "environment", "skills", "reminder", "thinking", "reasoning_summary",
	"tool_definition", "metadata",
}

// PromptRecordingPolicy explicitly chooses which readable content categories
// may be recorded. Encrypted and binary content is never eligible for recording.
type PromptRecordingPolicy struct {
	Version int             `json:"version"`
	Keep    map[string]bool `json:"keep"`
}

// UnmarshalJSON rejects null/non-boolean switches rather than treating them as
// false; updates must explicitly specify every category's intended value.
func (p *PromptRecordingPolicy) UnmarshalJSON(data []byte) error {
	var raw struct {
		Version int                        `json:"version"`
		Keep    map[string]json.RawMessage `json:"keep"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	policy := PromptRecordingPolicy{Version: raw.Version, Keep: make(map[string]bool, len(raw.Keep))}
	for category, value := range raw.Keep {
		if string(bytes.TrimSpace(value)) != "true" && string(bytes.TrimSpace(value)) != "false" {
			return fmt.Errorf("recording_policy.keep.%s must be a boolean", category)
		}
		policy.Keep[category] = string(bytes.TrimSpace(value)) == "true"
	}
	*p = policy
	return nil
}

func RecommendedPromptRecordingPolicy() PromptRecordingPolicy {
	policy := PromptRecordingPolicy{Version: promptRecordingPolicyVersion, Keep: make(map[string]bool, len(promptRecordingPolicyCategories))}
	for _, category := range promptRecordingPolicyCategories {
		policy.Keep[category] = category == "user" || category == "assistant" || category == "tool_call" || category == "tool_result"
	}
	return policy
}

func ValidatePromptRecordingPolicy(policy PromptRecordingPolicy) error {
	if policy.Version != promptRecordingPolicyVersion {
		return fmt.Errorf("recording_policy.version must be %d", promptRecordingPolicyVersion)
	}
	for category := range policy.Keep {
		known := false
		for _, expected := range promptRecordingPolicyCategories {
			if category == expected {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("recording_policy.keep contains unknown category %q", category)
		}
	}
	for _, category := range promptRecordingPolicyCategories {
		if _, present := policy.Keep[category]; !present {
			return fmt.Errorf("recording_policy.keep must include category %q", category)
		}
	}
	return nil
}

func ClonePromptRecordingPolicy(policy *PromptRecordingPolicy) *PromptRecordingPolicy {
	if policy == nil {
		return nil
	}
	copy := &PromptRecordingPolicy{Version: policy.Version, Keep: make(map[string]bool, len(policy.Keep))}
	for category, keep := range policy.Keep {
		copy.Keep[category] = keep
	}
	return copy
}

func clonePromptRecordingConfig(config PromptRecordingConfig) PromptRecordingConfig {
	config.RecordingPolicy = ClonePromptRecordingPolicy(config.RecordingPolicy)
	return config
}

func defaultPromptRecordingConfig() PromptRecordingConfig {
	policy := RecommendedPromptRecordingPolicy()
	return PromptRecordingConfig{
		Enabled: true, HeadersEnabled: true, PromptEnabled: true, ResponseEnabled: true,
		MaxMessages: promptRecordDefaultMaxMessages, RecordingPolicy: &policy,
	}
}

func validatePromptRecordingSettingsUpdate(update PromptRecordingSettingsUpdate) error {
	if update.RecordingPolicy != nil {
		return ValidatePromptRecordingPolicy(*update.RecordingPolicy)
	}
	return nil
}
