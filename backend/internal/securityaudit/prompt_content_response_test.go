package securityaudit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func policyResponseRequest() Request {
	policy := RecommendedPromptRecordingPolicy()
	return Request{Protocol: "responses", recordingConfig: &PromptRecordingConfig{Enabled: true, ResponseEnabled: true, RecordingPolicy: &policy}}
}

func TestRecordedResponsePolicyAcrossJSONProtocols(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"responses", `{"output":[{"type":"reasoning","encrypted_content":"CIPHER","summary":[{"type":"summary_text","text":"UNWANTED_THINKING"}]},{"role":"assistant","content":[{"type":"output_text","text":"Visible answer"}]},{"type":"function_call","call_id":"call-1","name":"read","arguments":"{\"file\":\"main.go\"}"}]}`},
		{"chat", `{"choices":[{"message":{"role":"assistant","content":"Visible answer","reasoning_content":"UNWANTED_THINKING","tool_calls":[{"id":"call-1","type":"function","function":{"name":"read","arguments":"{\"file\":\"main.go\"}"}}]}}]}`},
		{"claude", `{"content":[{"type":"thinking","thinking":"UNWANTED_THINKING","signature":"CIPHER"},{"type":"text","text":"Visible answer"},{"type":"tool_use","id":"call-1","name":"read","input":{"file":"main.go"}}]}`},
		{"gemini", `{"candidates":[{"content":{"role":"model","parts":[{"thought":true,"text":"UNWANTED_THINKING","thoughtSignature":"CIPHER"},{"text":"Visible answer"},{"functionCall":{"name":"read","args":{"file":"main.go"}}}]}}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, ok := ExtractRecordedResponse(policyResponseRequest(), []byte(tc.body), false)
			require.True(t, ok)
			require.Contains(t, response.Text, "Visible answer")
			require.Contains(t, response.Text, "main.go")
			require.NotContains(t, response.Text, "UNWANTED_THINKING")
			require.NotContains(t, response.Document, "CIPHER")
			require.Contains(t, response.Document, `"schema_version":2`)
		})
	}
}

func TestRecordedResponseSSEAssemblesCallsAndAvoidsFinalDuplication(t *testing.T) {
	for _, body := range []string{
		"data: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"delta\":\"Visible \"}\n\ndata: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"delta\":\"answer\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"role\":\"assistant\",\"content\":\"Visible answer\"},{\"type\":\"function_call\",\"name\":\"read\",\"arguments\":\"main.go\"}]}}\n\n",
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Visible \",\"reasoning_content\":\"UNWANTED_THINKING\",\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"read\",\"arguments\":\"main\"}}]}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\",\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\".go\"}}]}}]}\n\ndata: [DONE]\n",
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Visible answer\"}}\n\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"c1\",\"name\":\"read\",\"input\":{}}}\n\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"main.go\"}}\n",
	} {
		response, ok := ExtractRecordedResponse(policyResponseRequest(), []byte(body), false)
		require.True(t, ok)
		require.Equal(t, 1, strings.Count(response.Text, "Visible answer"))
		require.Contains(t, response.Text, "main.go")
		require.NotContains(t, response.Text, "UNWANTED_THINKING")
	}
}

func TestRecordedResponsePolicySnapshotAndLimits(t *testing.T) {
	req := policyResponseRequest()
	req.recordingConfig.RecordingPolicy.Keep["thinking"] = true
	response, ok := ExtractRecordedResponse(req, []byte(`{"content":[{"type":"thinking","thinking":"retained thought","signature":"CIPHER"}]}`), false)
	require.True(t, ok)
	require.Contains(t, response.Text, "retained thought")
	require.NotContains(t, response.Document, "CIPHER")
	body, _ := json.Marshal(map[string]any{"output_text": strings.Repeat("你", PromptResponseTextLimit)})
	response, ok = ExtractRecordedResponse(req, body, true)
	require.True(t, ok)
	require.True(t, response.Truncated)
	require.LessOrEqual(t, len(response.Document), PromptResponseTextLimit)
	require.True(t, json.Valid([]byte(response.Document)))
}

func TestRecordingSnapshotSurvivesSettingsChange(t *testing.T) {
	settings := &promptRecordingSettingRepository{}
	manager := NewConfigManager(nil, settings, nil, prefixEncryptor{}, testTotpKeyConfig())
	require.NoError(t, manager.Reload(context.Background()))
	policy := RecommendedPromptRecordingPolicy()
	require.NoError(t, manager.SavePromptRecordingSettings(context.Background(), PromptRecordingSettingsUpdate{RecordingPolicy: &policy}))
	repo := &capturedRequestRepository{records: make(chan *PromptRecord, 1)}
	service := &PromptService{config: manager, records: newPromptRecordService(repo, 1, 1, 1)}
	coordinator := NewCoordinator(nil, nil, service)
	req, reference := coordinator.PreparePromptRecording(Request{Endpoint: "/v1/responses", Protocol: "responses", Body: []byte(`{"input":"hello"}`)})
	// Simulate an administrator updating settings while the upstream is running.
	policy.Keep["assistant"] = false
	require.NoError(t, manager.SavePromptRecordingSettings(context.Background(), PromptRecordingSettingsUpdate{RecordingPolicy: &policy}))
	service.RecordPrompt(context.Background(), req)
	select {
	case record := <-repo.records:
		require.Contains(t, record.RecordingPolicy, `"assistant":true`)
	case <-time.After(time.Second):
		t.Fatal("record not saved")
	}
	captured, ok := ExtractRecordedResponse(reference, []byte(`{"output_text":"still retained"}`), false)
	require.True(t, ok)
	require.Equal(t, "still retained", captured.Text)
	stopRecordService(t, service.records)
}

func TestRecordedResponseSSEPreservesLongGeminiText(t *testing.T) {
	var stream strings.Builder
	for i := 0; i < 80; i++ {
		event, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"index": 0, "content": map[string]any{"parts": []any{map[string]any{"text": "字"}}}}}})
		stream.WriteString("data: " + string(event) + "\n\n")
	}
	response, ok := ExtractRecordedResponse(policyResponseRequest(), []byte(stream.String()), false)
	require.True(t, ok)
	require.Equal(t, strings.Repeat("字", 80), response.Text)
	require.False(t, response.Truncated)
}

func TestRecordedResponseSSEReasoningSummarySwitchAndFinalItem(t *testing.T) {
	req := policyResponseRequest()
	req.recordingConfig.RecordingPolicy.Keep["reasoning_summary"] = true
	prefix := "data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"reasoning\",\"id\":\"r1\",\"summary\":[]}}\n\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"item_id\":\"r1\",\"delta\":\"retained summary\"}\n\n"
	response, ok := ExtractRecordedResponse(req, []byte(prefix), false)
	require.True(t, ok)
	require.Equal(t, "retained summary", response.Text)
	final := prefix + "data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"type\":\"reasoning\",\"id\":\"r1\",\"summary\":[{\"text\":\"retained summary\"}]}}\n\n"
	response, ok = ExtractRecordedResponse(req, []byte(final), false)
	require.True(t, ok)
	require.Equal(t, "retained summary", response.Text)
	req.recordingConfig.RecordingPolicy.Keep["reasoning_summary"] = false
	response, ok = ExtractRecordedResponse(req, []byte(final), false)
	require.True(t, ok)
	require.Empty(t, response.Text)
}

func TestRecordedResponseSSERefusalAndMetadata(t *testing.T) {
	req := policyResponseRequest()
	req.recordingConfig.RecordingPolicy.Keep["metadata"] = true
	response, ok := ExtractRecordedResponse(req, []byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"refusal\":\"I cannot help\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"output_tokens\":3}}\n\n"), false)
	require.True(t, ok)
	require.Contains(t, response.Text, "I cannot help")
	require.Contains(t, response.Document, "output_tokens")
}
