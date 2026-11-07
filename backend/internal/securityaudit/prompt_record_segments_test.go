package securityaudit

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestRecordedContentIncludesReadableToolConversation(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, body string
		texts                []string
	}{
		{
			name: "responses", protocol: "responses",
			body:  `{"instructions":"You are Codex","input":[{"role":"user","content":"查看配置"},{"type":"function_call","name":"read_file","call_id":"call-1","arguments":"{\"path\":\"config.go\"}"},{"type":"function_call_output","call_id":"call-1","output":"配置文件已读取"},{"type":"custom_tool_call","name":"shell","input":"cat README.md"},{"type":"custom_tool_call_output","output":"项目说明已读取"},{"type":"reasoning","summary":[{"type":"summary_text","text":"已确认配置含义"}],"encrypted_content":"OPAQUE"},{"role":"assistant","content":[{"type":"output_text","text":"这是配置说明"}]}]}`,
			texts: []string{"查看配置", "read_file", "config.go", "配置文件已读取", "cat README.md", "项目说明已读取", "这是配置说明"},
		},
		{
			name: "chat", protocol: "openai_chat",
			body:  `{"messages":[{"role":"user","content":"读取工具文档"},{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"tool.md\"}"}}]},{"role":"tool","tool_call_id":"call-1","content":"工具文档内容"},{"role":"assistant","function_call":{"name":"lookup","arguments":"{\"query\":\"旧版工具\"}"}},{"role":"function","name":"lookup","content":"旧版读取结果"},{"role":"assistant","content":"文档说明如下"}]}`,
			texts: []string{"读取工具文档", "read_file", "tool.md", "工具文档内容", "旧版工具", "旧版读取结果", "文档说明如下"},
		},
		{
			name: "claude", protocol: "anthropic_messages",
			body:  `{"messages":[{"role":"user","content":"读取版本信息"},{"role":"assistant","content":[{"type":"thinking","thinking":"需要读取当前版本","signature":"OPAQUE"},{"type":"tool_use","id":"tool-1","name":"read_file","input":{"path":"VERSION"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":[{"type":"text","text":"版本为三"}]}]},{"role":"assistant","content":[{"type":"text","text":"当前版本为三"},{"type":"refusal","refusal":"拒绝公开隐私"}]}]}`,
			texts: []string{"读取版本信息", "read_file", "VERSION", "版本为三", "当前版本为三", "拒绝公开隐私"},
		},
		{
			name: "claude server tools", protocol: "anthropic_messages",
			body:  `{"messages":[{"role":"user","content":"搜索部署文档"},{"role":"assistant","content":[{"type":"server_tool_use","id":"server-1","name":"web_search","input":{"query":"部署说明"},"signature":"OPAQUE"},{"type":"web_search_tool_result","tool_use_id":"server-1","content":[{"type":"web_search_result","title":"部署指南","url":"https://example.test/deploy","encrypted_content":"OPAQUE"}]},{"type":"text","text":"已找到部署文档"}]}]}`,
			texts: []string{"搜索部署文档", "web_search", "部署说明", "部署指南", "https://example.test/deploy", "已找到部署文档"},
		},
		{
			name: "gemini", protocol: "gemini",
			body:  `{"contents":[{"role":"user","parts":[{"text":"读取状态"}]},{"role":"model","parts":[{"functionCall":{"name":"read_state","args":{"path":"state.json"}},"thoughtSignature":"OPAQUE"}]},{"role":"user","parts":[{"functionResponse":{"name":"read_state","response":{"state":"运行正常","count":9007199254740993,"enabled":true}}}]},{"role":"model","parts":[{"text":"服务正常"}]}]}`,
			texts: []string{"读取状态", "read_state", "state.json", "运行正常", "9007199254740993", "true", "服务正常"},
		},
		{
			name: "websocket", protocol: "responses_websocket",
			body:  `{"type":"response.create","response":{"input":[{"role":"user","content":"读取变更"},{"type":"function_call_output","output":"更新说明"},{"role":"assistant","content":"变更完成"}]}}`,
			texts: []string{"读取变更", "更新说明", "变更完成"},
		},
		{
			name: "batch", protocol: "gemini",
			body:  `{"requests":[{"contents":[{"role":"user","parts":[{"text":"读取批次"}]},{"role":"user","parts":[{"functionResponse":{"name":"read","response":{"text":"批次一"}}}]}]},{"contents":[{"role":"model","parts":[{"text":"批次二"}]}]}]}`,
			texts: []string{"读取批次", "批次一", "批次二"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{
				Protocol: tc.protocol, Body: []byte(tc.body),
			}
			audit, err := ExtractPromptSnapshot(req)
			require.NoError(t, err)
			prepared, err := preparePromptRecord(req)
			require.NoError(t, err)
			require.Equal(t, audit.PromptHash, prepared.OriginalPromptHash)
			for _, text := range tc.texts {
				require.Contains(t, prepared.StoredSnapshot.FullPrompt, text)
			}
			require.NotContains(t, prepared.StoredSnapshot.FullPrompt, "OPAQUE")
			require.Equal(t, utf8.RuneCountInString(prepared.StoredSnapshot.FullPrompt), prepared.StoredSnapshot.PromptLength)
			require.Positive(t, prepared.StoredSnapshot.MessageCount)
			require.Equal(t, tc.body, string(req.Body))
		})
	}
}

func TestRecordedContentUsesIndependentCategorySwitches(t *testing.T) {
	body := []byte(`{"instructions":"You are Codex","input":[{"role":"developer","content":"<skills_instructions>技能说明</skills_instructions>"},{"role":"user","content":"# AGENTS.md instructions\n<INSTRUCTIONS>项目说明</INSTRUCTIONS>\n请读取配置"},{"type":"function_call_output","output":"工具读取结果"},{"type":"reasoning","summary":[{"type":"summary_text","text":"推理摘要"}]}]}`)
	req := Request{Protocol: "responses", Body: body}
	prepared, err := preparePromptRecord(req)
	require.NoError(t, err)
	for _, expected := range []string{"请读取配置", "工具读取结果"} {
		require.Contains(t, prepared.StoredSnapshot.FullPrompt, expected)
	}
	for _, excluded := range []string{"技能说明", "项目说明", "推理摘要"} {
		require.NotContains(t, prepared.StoredSnapshot.FullPrompt, excluded)
	}
	policy := RecommendedPromptRecordingPolicy()
	for _, category := range []string{"skills", "project", "reasoning_summary"} {
		policy.Keep[category] = true
	}
	req.recordingConfig = &PromptRecordingConfig{RecordingPolicy: &policy}
	enabled, err := preparePromptRecord(req)
	require.NoError(t, err)
	for _, expected := range []string{"技能说明", "项目说明", "推理摘要", "请读取配置", "工具读取结果"} {
		require.Contains(t, enabled.StoredSnapshot.FullPrompt, expected)
	}
	require.Equal(t, prepared.OriginalPromptHash, enabled.OriginalPromptHash)
}

func TestRecordedContentIncludesToolOnlyRequestsWithoutChangingOriginalIdentity(t *testing.T) {
	body := []byte(`{"input":[{"type":"function_call_output","call_id":"read-1","output":"仅有工具读取结果"}]}`)
	req := Request{Protocol: "responses", Body: body}
	original := promptRecordSnapshot(req)
	prepared, err := preparePromptRecord(req)
	require.NoError(t, err)
	require.Equal(t, original.PromptHash, prepared.OriginalPromptHash)
	require.Contains(t, prepared.StoredSnapshot.FullPrompt, "仅有工具读取结果")
	require.Zero(t, prepared.StoredSnapshot.MessageCount)
}
