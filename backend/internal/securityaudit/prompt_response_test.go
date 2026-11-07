package securityaudit

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestExtractRecordedResponseJSONProtocolsWithoutSnapshot(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "openai chat", body: `{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`, want: "hello"},
		{name: "openai responses", body: `{"output":[{"type":"message","content":[{"type":"output_text","text":"world"}]}]}`, want: "world"},
		{name: "anthropic", body: `{"content":[{"type":"text","text":"claude"}]}`, want: "claude"},
		{name: "gemini", body: `{"candidates":[{"content":{"parts":[{"text":"gemini"}]}}]}`, want: "gemini"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, recognized := ExtractRecordedResponse(Request{}, []byte(test.body), false)
			require.True(t, recognized)
			require.Equal(t, test.want, result.Text)
			require.Equal(t, utf8.RuneCountInString(result.Text), result.Length)
			require.True(t, json.Valid([]byte(result.Document)))
			require.False(t, result.Truncated)
		})
	}
}

func TestExtractRecordedResponseSSEUsesDeltasWithoutDuplicatingFinalResponse(t *testing.T) {
	body := strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"hel"}`,
		``,
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"lo"}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"output":[{"content":[{"type":"output_text","text":"hello"}]}]}}`,
		``,
		`data: [DONE]`,
	}, "\n")
	result, recognized := ExtractRecordedResponse(Request{}, []byte(body), false)
	require.True(t, recognized)
	require.Equal(t, "hello", result.Text)
}

func TestExtractRecordedResponseMarksCaptureAndStorageTruncation(t *testing.T) {
	result, recognized := ExtractRecordedResponse(Request{}, []byte(`{"choices":[{"message":{"content":"hello"}}]}`), true)
	require.True(t, recognized)
	require.True(t, result.Truncated)

	longText := strings.Repeat("你", PromptResponseTextLimit)
	payload := `{"choices":[{"message":{"content":"` + longText + `"}}]}`
	result, recognized = ExtractRecordedResponse(Request{}, []byte(payload), false)
	require.True(t, recognized)
	require.True(t, result.Truncated)
	require.NotEmpty(t, result.Text)
	require.LessOrEqual(t, len(result.Text), PromptResponseTextLimit)
	require.LessOrEqual(t, len(result.Document), PromptResponseTextLimit)
	require.Equal(t, utf8.RuneCountInString(result.Text), result.Length)
	require.True(t, json.Valid([]byte(result.Document)))
	require.True(t, utf8.ValidString(result.Text))
}
