package securityaudit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHistoricalPromptRecordSegmentsRemainReadable(t *testing.T) {
	for _, body := range []string{
		`{"messages":[{"role":"user","content":"QUESTION"},{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"read_file","arguments":"{\"signature\":\"BUSINESS_VALUE\"}"}}]},{"role":"tool","tool_call_id":"c1","content":"RESULT"},{"role":"assistant","content":"ANSWER"}]}`,
		`{"response":{"input":[{"role":"user","content":"QUESTION"},{"type":"function_call","name":"read_file","arguments":{"signature":"BUSINESS_VALUE"}},{"type":"function_call_output","output":"RESULT"},{"role":"assistant","content":"ANSWER"}]}}`,
	} {
		document, err := decodePromptDocument([]byte(body))
		require.NoError(t, err)
		before, err := json.Marshal(document)
		require.NoError(t, err)
		segments := extractPromptRecordSegments("responses", document)
		texts := make([]string, 0, len(segments))
		for _, segment := range segments {
			texts = append(texts, segment.text)
		}
		text := strings.Join(texts, "\n")
		for _, expected := range []string{"QUESTION", "read_file", "BUSINESS_VALUE", "RESULT", "ANSWER"} {
			require.Contains(t, text, expected)
		}
		after, err := json.Marshal(document)
		require.NoError(t, err)
		require.Equal(t, before, after, "historical detail extraction is read-only")
	}
}
