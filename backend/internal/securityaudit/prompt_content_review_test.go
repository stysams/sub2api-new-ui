package securityaudit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// These cases exercise protocol fields whose nested payload shape differs
// from application-defined function arguments and results.
func TestContentReviewBuiltinComputerResultStripsScreenshot(t *testing.T) {
	input, err := decodePromptDocument([]byte(`{"input":[{"type":"computer_call_output","call_id":"c1","output":{"type":"computer_screenshot","image_url":"data:image/png;base64,OPAQUE_SCREENSHOT"}}]}`))
	require.NoError(t, err)
	document := AnalyzePromptContent("responses", input, RecommendedPromptRecordingPolicy(), 30)
	require.NotContains(t, PromptContentText(document), "OPAQUE_SCREENSHOT")
}

func TestContentReviewMetadataToggleRetainsResponseUsage(t *testing.T) {
	req := policyResponseRequest()
	req.recordingConfig.RecordingPolicy.Keep["metadata"] = true
	response, recognized := ExtractRecordedResponse(req, []byte(`{"output":[{"role":"assistant","content":"answer"}],"usage":{"output_tokens":3}}`), false)
	require.True(t, recognized)
	require.Contains(t, response.Document, "output_tokens")
}

func TestContentReviewImageProtocolRetainsNaturalPrompt(t *testing.T) {
	input, err := decodePromptDocument([]byte(`{"description":"draw a tree","image":"data:image/png;base64,OPAQUE_IMAGE"}`))
	require.NoError(t, err)
	document := AnalyzePromptContent("grok_media", input, RecommendedPromptRecordingPolicy(), 30)
	require.Contains(t, PromptContentText(document), "draw a tree")
	require.NotContains(t, PromptContentText(document), "OPAQUE_IMAGE")
}

func TestContentReviewChatAssistantAnalysisChannelIsThinking(t *testing.T) {
	input, err := decodePromptDocument([]byte(`{"messages":[{"role":"assistant","channel":"analysis","content":"INTERNAL_REASONING"},{"role":"assistant","channel":"final","content":"visible answer"}]}`))
	require.NoError(t, err)
	document := AnalyzePromptContent("responses", input, RecommendedPromptRecordingPolicy(), 30)
	require.Contains(t, PromptContentText(document), "visible answer")
	require.False(t, strings.Contains(PromptContentText(document), "INTERNAL_REASONING"))
}

func TestContentReviewResponseTextBoundIncludesRenderedToolData(t *testing.T) {
	var value any = "leaf"
	for range 700 {
		value = map[string]any{"nested": value}
	}
	body, err := json.Marshal(map[string]any{"output": []any{map[string]any{
		"type": "function_call", "name": "example", "arguments": value,
	}}})
	require.NoError(t, err)
	response, recognized := ExtractRecordedResponse(policyResponseRequest(), body, false)
	require.True(t, recognized)
	require.LessOrEqual(t, len(response.Text), PromptResponseTextLimit)
}

func TestContentReviewAdditionalToolsSwitchIndependentOfDeveloper(t *testing.T) {
	input, err := decodePromptDocument([]byte(`{"input":[{"type":"additional_tools","role":"developer","tools":[{"type":"function","name":"read","description":"TOOL_DESCRIPTION"}]},{"role":"user","content":"question"}]}`))
	require.NoError(t, err)
	policy := RecommendedPromptRecordingPolicy()
	policy.Keep["tool_definition"] = true
	document := AnalyzePromptContent("responses", input, policy, 30)
	require.Contains(t, PromptContentText(document), "TOOL_DESCRIPTION")
	require.Equal(t, 1, PromptContentMessageCount(document))
	policy.Keep["tool_definition"] = false
	policy.Keep["developer"] = true
	document = AnalyzePromptContent("responses", input, policy, 30)
	require.NotContains(t, PromptContentText(document), "TOOL_DESCRIPTION")
}

func TestContentReviewToolSearchOutputUsesToolResultSwitch(t *testing.T) {
	input, err := decodePromptDocument([]byte(`{"input":[{"type":"tool_search_output","call_id":"call_1","output":"SEARCH_RESULT"}]}`))
	require.NoError(t, err)
	policy := RecommendedPromptRecordingPolicy()
	policy.Keep["user"] = false
	document := AnalyzePromptContent("responses", input, policy, 30)
	require.Contains(t, PromptContentText(document), "SEARCH_RESULT")
}

func TestContentReviewCompactionSummaryUsesSummarySwitch(t *testing.T) {
	input, err := decodePromptDocument([]byte(`{"input":[{"type":"compaction_summary","summary":[{"type":"summary_text","text":"COMPACTION_SUMMARY"}],"encrypted_content":"CIPHER"}]}`))
	require.NoError(t, err)
	policy := RecommendedPromptRecordingPolicy()
	document := AnalyzePromptContent("responses", input, policy, 30)
	require.NotContains(t, PromptContentText(document), "COMPACTION_SUMMARY")
	policy.Keep["reasoning_summary"] = true
	policy.Keep["user"] = false
	policy.Keep["assistant"] = false
	document = AnalyzePromptContent("responses", input, policy, 30)
	require.Contains(t, PromptContentText(document), "COMPACTION_SUMMARY")
	require.NotContains(t, PromptContentText(document), "CIPHER")
}
