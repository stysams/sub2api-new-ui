package securityaudit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func contentTestPolicyAll() PromptRecordingPolicy {
	policy := RecommendedPromptRecordingPolicy()
	for category := range policy.Keep {
		policy.Keep[category] = true
	}
	// Even an unvalidated caller cannot retain opaque/media categories.
	policy.Keep["encrypted"] = true
	policy.Keep["media"] = true
	return policy
}

func contentTestDecode(t *testing.T, body string) any {
	t.Helper()
	document, err := decodePromptDocument([]byte(body))
	require.NoError(t, err)
	return document
}

func TestPromptContentDocumentProtocolBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, body string
		want                 []string
	}{
		{
			name: "responses", protocol: "responses",
			body: `{"instructions":"You are Codex","input":[{"role":"user","content":"读取文件"},{"type":"function_call","name":"read_file","call_id":"c1","arguments":"{\"metadata\":\"business metadata\",\"signature\":\"business signature\",\"file\":\"business file\",\"type\":\"thinking\"}"},{"type":"function_call_output","call_id":"c1","output":{"metadata":"business metadata","signature":"business signature","file":"business file","type":"thinking","encrypted_content":"business encrypted field","messages":[1,2,3]}},{"type":"reasoning","summary":[{"type":"summary_text","text":"摘要可读"}],"encrypted_content":"PROTOCOL_SECRET"},{"role":"assistant","content":[{"type":"output_text","text":"读取完毕"}]}]}`,
			want: []string{"读取文件", "read_file", "business metadata", "business signature", "business file", "thinking", "business encrypted field", "摘要可读", "读取完毕"},
		},
		{
			name: "chat", protocol: "openai_chat",
			body: `{"messages":[{"role":"user","content":"读取文件"},{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"read_file","arguments":"{\"signature\":\"business signature\"}"}}]},{"role":"tool","tool_call_id":"c1","content":"<skills_instructions>这是工具原文</skills_instructions>"},{"role":"assistant","content":"读取完毕","reasoning_content":"思考可读","signature":"PROTOCOL_SECRET"}]}`,
			want: []string{"读取文件", "read_file", "business signature", "<skills_instructions>这是工具原文</skills_instructions>", "读取完毕", "思考可读"},
		},
		{
			name: "claude", protocol: "anthropic_messages",
			body: `{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"思考可读","signature":"PROTOCOL_SECRET"},{"type":"tool_use","id":"c1","name":"read_file","input":{"metadata":"business metadata","signature":"business signature","file":"business file","type":"thinking"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"c1","content":[{"type":"thinking","signature":"business signature","file":"business file","metadata":"business metadata"},{"type":"text","text":"工具自然结果"},{"type":"image","source":{"data":"PROTOCOL_MEDIA"}}]}]},{"role":"assistant","content":[{"type":"text","text":"读取完毕"},{"type":"refusal","refusal":"拒绝公开隐私"}]}]}`,
			want: []string{"read_file", "思考可读", "business metadata", "business signature", "business file", "thinking", "工具自然结果", "读取完毕", "拒绝公开隐私"},
		},
		{
			name: "gemini", protocol: "gemini",
			body: `{"contents":[{"role":"user","parts":[{"text":"读取文件"}]},{"role":"model","parts":[{"functionCall":{"id":"c1","name":"read_file","args":{"metadata":"business metadata","signature":"business signature","file":"business file","type":"thinking","count":9007199254740993}},"thoughtSignature":"PROTOCOL_SECRET"}]},{"role":"user","parts":[{"functionResponse":{"id":"c1","response":{"type":"thinking","file":"business file"}}}]},{"role":"model","parts":[{"text":"读取完毕"},{"inlineData":{"mimeType":"image/png","data":"PROTOCOL_MEDIA"}}]}]}`,
			want: []string{"读取文件", "read_file", "business metadata", "business signature", "business file", "thinking", "9007199254740993", "读取完毕"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := contentTestDecode(t, tc.body)
			before, err := json.Marshal(input)
			require.NoError(t, err)
			document := AnalyzePromptContent(tc.protocol, input, contentTestPolicyAll(), 30)
			text := PromptContentText(document)
			for _, expected := range tc.want {
				require.Contains(t, text, expected)
			}
			encoded, err := json.Marshal(document)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "PROTOCOL_SECRET")
			require.NotContains(t, string(encoded), "PROTOCOL_MEDIA")
			require.Positive(t, document.Filtered["encrypted"].Count)
			after, err := json.Marshal(input)
			require.NoError(t, err)
			require.Equal(t, before, after, "analysis must not mutate the request")
			if tc.name == "gemini" {
				for _, block := range document.Blocks {
					if block.Category == "tool_result" {
						require.Equal(t, "c1", block.CallID)
						require.Equal(t, "read_file", block.Name)
					}
				}
			}
		})
	}
}

func TestPromptContentDocumentIndependentSwitches(t *testing.T) {
	input := contentTestDecode(t, `{"instructions":"instruction-marker","tools":[{"type":"function","name":"tool-definition-marker"}],"metadata":{"label":"metadata-marker"},"messages":[{"role":"system","content":"system-marker"},{"role":"developer","content":"You are Codex\ndeveloper-marker\n<environment_context>environment-marker</environment_context><skills_instructions>skills-marker</skills_instructions><system-reminder>reminder-marker</system-reminder>\n# AGENTS.md instructions\n<INSTRUCTIONS>project-marker</INSTRUCTIONS>"},{"role":"user","content":"user-marker"},{"role":"assistant","content":[{"type":"thinking","thinking":"thinking-marker","signature":"SECRET"},{"type":"reasoning","summary":[{"type":"summary_text","text":"summary-marker"}]},{"type":"text","text":"assistant-marker"},{"type":"tool_use","id":"c1","name":"call-marker","input":{"path":"a"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"c1","content":"result-marker"}]}]}`)
	markers := map[string]string{
		"user": "user-marker", "assistant": "assistant-marker", "tool_call": "call-marker", "tool_result": "result-marker",
		"system": "system-marker", "developer": "developer-marker", "instructions": "instruction-marker", "agent": "You are Codex",
		"project": "project-marker", "environment": "environment-marker", "skills": "skills-marker", "reminder": "reminder-marker",
		"thinking": "thinking-marker", "reasoning_summary": "summary-marker", "tool_definition": "tool-definition-marker", "metadata": "metadata-marker",
	}
	for category, marker := range markers {
		t.Run(category, func(t *testing.T) {
			policy := RecommendedPromptRecordingPolicy()
			for key := range policy.Keep {
				policy.Keep[key] = key == category
			}
			document := AnalyzePromptContent("responses", input, policy, 30)
			require.Contains(t, PromptContentText(document), marker)
			for _, block := range document.Blocks {
				require.Equal(t, category, block.Category)
			}
			require.NotContains(t, PromptContentText(document), "SECRET")
		})
	}
}

func TestPromptContentDocumentTaggedOrderAndQuotedText(t *testing.T) {
	source := "before\n<environment_context>ENV</environment_context><skills_instructions>SKILL</skills_instructions>after\n"
	document := AnalyzePromptContent("responses", map[string]any{"messages": []any{map[string]any{"role": "developer", "content": source}}}, contentTestPolicyAll(), 30)
	categories, texts := []string{}, []string{}
	for _, block := range document.Blocks {
		categories = append(categories, block.Category)
		texts = append(texts, block.Text)
	}
	require.Equal(t, []string{"developer", "environment", "skills", "developer"}, categories)
	require.Equal(t, source, strings.Join(texts, ""))

	quoted := "```xml\n<skills_instructions>quoted skill</skills_instructions>\n```\n> <environment_context>quoted environment</environment_context>\n"
	for _, identity := range []string{"You are Codex", "```text\nYou are Codex\n```", "You are a helpful assistant"} {
		input := map[string]any{"instructions": identity, "messages": []any{
			map[string]any{"role": "developer", "content": quoted},
			map[string]any{"role": "user", "content": quoted},
		}}
		result := AnalyzePromptContent("responses", input, contentTestPolicyAll(), 30)
		for _, block := range result.Blocks {
			if block.Role == "developer" || block.Role == "user" {
				require.Equal(t, quoted, block.Text)
				require.Equal(t, block.Role, block.Category)
			}
		}
	}
}

func TestPromptContentDocumentUserInjectionRequiresKnownIdentityAndPrefix(t *testing.T) {
	injection := "# AGENTS.md instructions\n<INSTRUCTIONS>PROJECT</INSTRUCTIONS><environment_context>ENV</environment_context>实际问题"
	for _, tc := range []struct {
		identity string
		split    bool
	}{
		{"You are Codex", true}, {"You are a helpful assistant", false}, {"```text\nYou are Codex\n```", false},
	} {
		input := map[string]any{"instructions": tc.identity, "input": []any{map[string]any{"role": "user", "content": injection}}}
		result := AnalyzePromptContent("responses", input, RecommendedPromptRecordingPolicy(), 30)
		if tc.split {
			require.Equal(t, "实际问题", PromptContentText(result))
			require.Equal(t, 1, result.Filtered["project"].Count)
			require.Equal(t, 1, result.Filtered["environment"].Count)
		} else {
			require.Equal(t, injection, PromptContentText(result))
		}
	}
	input := map[string]any{"instructions": "You are Codex", "input": []any{map[string]any{"role": "user", "content": "请解释这个片段：\n" + injection}}}
	require.Equal(t, "请解释这个片段：\n"+injection, PromptContentText(AnalyzePromptContent("responses", input, RecommendedPromptRecordingPolicy(), 30)))
}

func TestPromptContentDocumentMessageLimitPreservesBusinessArraysAndCallAssociation(t *testing.T) {
	input := contentTestDecode(t, `{"messages":[{"role":"user","content":"old-user"},{"role":"user","content":"latest-user"},{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"read_file","arguments":"{}"}}]},{"role":"tool","tool_call_id":"c1","content":{"messages":[1,2,3,4,5],"input":[1,2,3,4,5],"signature":"BUSINESS"}},{"role":"assistant","content":"latest-assistant"}]}`)
	document := AnalyzePromptContent("openai_chat", input, RecommendedPromptRecordingPolicy(), 2)
	require.True(t, document.Truncated)
	require.Equal(t, 2, PromptContentMessageCount(document))
	require.NotContains(t, PromptContentText(document), "old-user")
	require.Len(t, document.Blocks, 4)
	require.Equal(t, "latest-user", document.Blocks[0].Text)
	require.Equal(t, "tool_call", document.Blocks[1].Category)
	result := document.Blocks[2]
	require.Equal(t, "tool_result", result.Category)
	require.Equal(t, "c1", result.CallID)
	require.Equal(t, "read_file", result.Name)
	data := result.Data.(map[string]any)["content"].(map[string]any)
	require.Len(t, data["messages"], 5)
	require.Len(t, data["input"], 5)
	require.Equal(t, "BUSINESS", data["signature"])

	withReasoning := contentTestDecode(t, `{"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"SUMMARY"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ANSWER"}]}]}`)
	resultDocument := AnalyzePromptContent("responses", withReasoning, contentTestPolicyAll(), 1)
	require.False(t, resultDocument.Truncated)
	require.Equal(t, 1, PromptContentMessageCount(resultDocument))
	require.Contains(t, PromptContentText(resultDocument), "SUMMARY")
}

func TestPromptContentDocumentMessageLimitCountsOnlyDialogue(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, body string
	}{
		{
			name: "chat", protocol: "openai_chat",
			body: `{"messages":[{"role":"system","content":"SYSTEM"},{"role":"developer","content":"DEVELOPER"},{"role":"user","content":"OLD_USER"},{"role":"assistant","tool_calls":[{"id":"old","type":"function","function":{"name":"read","arguments":"OLD_CALL"}}]},{"role":"tool","tool_call_id":"old","content":"OLD_RESULT"},{"role":"user","content":"LATEST_USER"},{"role":"assistant","channel":"analysis","content":"THINKING"},{"role":"assistant","tool_calls":[{"id":"latest","type":"function","function":{"name":"read","arguments":"LATEST_CALL"}}]},{"role":"tool","tool_call_id":"latest","content":"LATEST_RESULT"},{"role":"assistant","content":[{"type":"text","text":"LATEST_ANSWER"},{"type":"text","text":"ANSWER_PART_TWO"}],"tool_calls":[{"id":"tail","type":"function","function":{"name":"read","arguments":"TAIL_CALL"}}]},{"role":"tool","tool_call_id":"tail","content":"TAIL_RESULT"}]}`,
		},
		{
			name: "responses", protocol: "responses",
			body: `{"instructions":"SYSTEM","input":[{"role":"user","content":"OLD_USER"},{"type":"function_call","call_id":"old","name":"read","arguments":"OLD_CALL"},{"type":"function_call_output","call_id":"old","output":"OLD_RESULT"},{"role":"user","content":"LATEST_USER"},{"type":"reasoning","summary":[{"type":"summary_text","text":"THINKING"}]},{"type":"function_call","call_id":"latest","name":"read","arguments":"LATEST_CALL"},{"type":"function_call_output","call_id":"latest","output":"LATEST_RESULT"},{"role":"assistant","content":[{"type":"output_text","text":"LATEST_ANSWER"},{"type":"output_text","text":"ANSWER_PART_TWO"}]},{"type":"function_call","call_id":"tail","name":"read","arguments":"TAIL_CALL"},{"type":"function_call_output","call_id":"tail","output":"TAIL_RESULT"}]}`,
		},
		{
			name: "claude", protocol: "anthropic",
			body: `{"system":"SYSTEM","messages":[{"role":"user","content":"OLD_USER"},{"role":"assistant","content":[{"type":"tool_use","id":"old","name":"read","input":"OLD_CALL"}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"old","content":"OLD_RESULT"}]},{"role":"user","content":"LATEST_USER"},{"role":"assistant","content":[{"type":"thinking","thinking":"THINKING"}]},{"role":"assistant","content":[{"type":"tool_use","id":"latest","name":"read","input":"LATEST_CALL"}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"latest","content":"LATEST_RESULT"}]},{"role":"assistant","content":[{"type":"text","text":"LATEST_ANSWER"},{"type":"text","text":"ANSWER_PART_TWO"},{"type":"tool_use","id":"tail","name":"read","input":"TAIL_CALL"}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tail","content":"TAIL_RESULT"}]}]}`,
		},
		{
			name: "gemini", protocol: "gemini",
			body: `{"systemInstruction":{"parts":[{"text":"SYSTEM"}]},"contents":[{"role":"user","parts":[{"text":"OLD_USER"}]},{"role":"model","parts":[{"functionCall":{"name":"read","args":"OLD_CALL"}}]},{"role":"user","parts":[{"functionResponse":{"name":"read","response":"OLD_RESULT"}}]},{"role":"user","parts":[{"text":"LATEST_USER"}]},{"role":"model","parts":[{"text":"THINKING","thought":true}]},{"role":"model","parts":[{"functionCall":{"name":"read","args":"LATEST_CALL"}}]},{"role":"user","parts":[{"functionResponse":{"name":"read","response":"LATEST_RESULT"}}]},{"role":"model","parts":[{"text":"LATEST_ANSWER"},{"text":"ANSWER_PART_TWO"},{"functionCall":{"name":"read","args":"TAIL_CALL"}}]},{"role":"user","parts":[{"functionResponse":{"name":"read","response":"TAIL_RESULT"}}]}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := contentTestDecode(t, tc.body)
			unlimited := AnalyzePromptContent(tc.protocol, input, contentTestPolicyAll(), -1)
			require.Equal(t, 3, PromptContentMessageCount(unlimited))
			encoded, err := json.Marshal(unlimited)
			require.NoError(t, err)
			for _, source := range []any{input, unlimited, contentTestDecode(t, string(encoded))} {
				for _, disabled := range [][]string{nil, {"user"}, {"assistant"}, {"user", "assistant"}} {
					policy := contentTestPolicyAll()
					for _, category := range disabled {
						policy.Keep[category] = false
					}
					document := AnalyzePromptContent(tc.protocol, source, policy, 2)
					require.True(t, document.Truncated)
					require.Equal(t, 2-len(disabled), PromptContentMessageCount(document))
					text := PromptContentText(document)
					for _, marker := range []string{"OLD_USER", "OLD_CALL", "OLD_RESULT"} {
						require.NotContains(t, text, marker)
					}
					for _, marker := range []string{"LATEST_CALL", "LATEST_RESULT", "TAIL_CALL", "TAIL_RESULT", "THINKING"} {
						require.Contains(t, text, marker)
					}
					if policy.Keep["user"] {
						require.Contains(t, text, "LATEST_USER")
					}
					if policy.Keep["assistant"] {
						require.Contains(t, text, "LATEST_ANSWER")
						require.Contains(t, text, "ANSWER_PART_TWO")
					}
				}
			}
		})
	}
}

func TestPromptContentDocumentToolOnlyAndEmptyMessagesDoNotConsumeSlots(t *testing.T) {
	input := contentTestDecode(t, `{"messages":[{"role":"system","content":"SYSTEM"},{"role":"developer","content":"DEVELOPER"},{"role":"user","content":""},{"role":"assistant","content":"  \n"},{"role":"assistant","channel":"analysis","content":"THINKING"},{"role":"assistant","tool_calls":[{"id":"read","type":"function","function":{"name":"read","arguments":"{}"}}]},{"role":"tool","tool_call_id":"read","content":"RESULT"},{"role":"assistant","content":[{"type":"thinking","thinking":"MORE_THINKING"}]}]}`)
	document := AnalyzePromptContent("openai_chat", input, contentTestPolicyAll(), 1)
	require.False(t, document.Truncated)
	require.Zero(t, PromptContentMessageCount(document))
	for _, marker := range []string{"SYSTEM", "DEVELOPER", "THINKING", "MORE_THINKING", "RESULT", "read"} {
		require.Contains(t, PromptContentText(document), marker)
	}
}

func TestPromptContentDocumentBuiltinMediaAndAudio(t *testing.T) {
	input := contentTestDecode(t, `{"input":[{"type":"computer_call_output","call_id":"c1","output":{"type":"computer_screenshot","image_url":"MEDIA_COMPUTER"}},{"type":"function_call_output","call_id":"c2","output":{"type":"computer_screenshot","image_url":"BUSINESS_IMAGE_URL","signature":"BUSINESS_SIGNATURE"}},{"type":"code_interpreter_call","code":"print(1)","outputs":[{"type":"logs","logs":"LOG_TEXT"},{"type":"image","url":"MEDIA_CODE_IMAGE"}]},{"role":"assistant","audio":{"id":"audio1","data":"MEDIA_AUDIO","transcript":"AUDIO_TRANSCRIPT"},"content":"ANSWER"},{"type":"web_search_tool_result","tool_use_id":"server1","content":[{"type":"web_search_result","title":"SEARCH_TITLE","encrypted_content":"SEARCH_SECRET"}]}]}`)
	document := AnalyzePromptContent("responses", input, contentTestPolicyAll(), 30)
	text := PromptContentText(document)
	for _, marker := range []string{"BUSINESS_IMAGE_URL", "BUSINESS_SIGNATURE", "LOG_TEXT", "AUDIO_TRANSCRIPT", "ANSWER", "SEARCH_TITLE"} {
		require.Contains(t, text, marker)
	}
	for _, marker := range []string{"MEDIA_COMPUTER", "MEDIA_CODE_IMAGE", "MEDIA_AUDIO", "SEARCH_SECRET"} {
		require.NotContains(t, text, marker)
	}
	require.Equal(t, 3, document.Filtered["media"].Count)
	require.Equal(t, 1, document.Filtered["encrypted"].Count)
}

func TestPromptContentDocumentRefilterAndSchemaValidation(t *testing.T) {
	input := contentTestDecode(t, `{"input":[{"role":"user","content":"USER"},{"role":"assistant","content":[{"type":"thinking","thinking":"THINKING","signature":"SECRET"},{"type":"text","text":"ANSWER"}]}]}`)
	initial := AnalyzePromptContent("responses", input, contentTestPolicyAll(), 30)
	encoded, err := json.Marshal(initial)
	require.NoError(t, err)
	refiltered := AnalyzePromptContent("responses", contentTestDecode(t, string(encoded)), RecommendedPromptRecordingPolicy(), 30)
	require.Equal(t, "USER\n\nANSWER", PromptContentText(refiltered))
	require.Equal(t, 1, refiltered.Filtered["thinking"].Count)
	require.Equal(t, 1, refiltered.Filtered["encrypted"].Count)
	require.Equal(t, len("THINKING"), refiltered.Filtered["thinking"].Bytes)
	require.Contains(t, PromptContentText(initial), "THINKING")
	for _, body := range []string{
		`{"schema_version":99,"blocks":[{"category":"user","text":"BUSINESS"}],"filtered":{}}`,
		`{"schema_version":2,"blocks":[{"category":"business-category","text":"BUSINESS"}],"filtered":{}}`,
		`{"schema_version":2,"blocks":[{"category":"user","text":"BUSINESS"}],"filtered":{},"business":true}`,
	} {
		result := AnalyzePromptContent("responses", contentTestDecode(t, body), contentTestPolicyAll(), 30)
		require.Contains(t, PromptContentText(result), "BUSINESS")
		for _, block := range result.Blocks {
			require.Equal(t, "metadata", block.Category, "business JSON must not impersonate a normalized document")
		}
	}
}

func TestPromptContentDocumentResponseAndBatchEnvelopes(t *testing.T) {
	for _, body := range []string{
		`{"request":{"messages":[{"role":"user","content":"QUESTION"}]},"response":{"choices":[{"message":{"role":"assistant","content":"ANSWER"}}]}}`,
		`{"requests":[{"contents":[{"role":"user","parts":[{"text":"QUESTION"}]}]},{"candidates":[{"content":{"role":"model","parts":[{"text":"ANSWER"}]}}]}]}`,
		`{"type":"response.create","response":{"input":[{"role":"user","content":"QUESTION"},{"role":"assistant","content":"ANSWER"}]}}`,
	} {
		result := AnalyzePromptContent("responses", contentTestDecode(t, body), RecommendedPromptRecordingPolicy(), 30)
		require.Equal(t, "QUESTION\n\nANSWER", PromptContentText(result))
		require.Equal(t, 2, PromptContentMessageCount(result))
	}
}

func TestPromptContentDocumentCarrierAndCompactionItems(t *testing.T) {
	input := contentTestDecode(t, `{"input":[{"type":"additional_tools","role":"developer","tools":[{"name":"DEFINITION"}]},{"type":"additional_tools","tools":[{"name":"DEFINITION_TWO"}]},{"type":"item_reference","id":"item1"},{"type":"computer_initialize_state","state":"INIT_STATE"},{"type":"compaction_trigger","value":"TRIGGER"},{"type":"tool_search_output","call_id":"search1","output":"SEARCH_RESULT"},{"type":"compaction_summary","summary":[{"type":"summary_text","text":"SUMMARY"}],"encrypted_content":"COMPACTION_SECRET"},{"role":"assistant","content":null,"refusal":"REFUSAL"}]}`)
	result := AnalyzePromptContent("responses", input, RecommendedPromptRecordingPolicy(), 30)
	require.Equal(t, "tool_result", result.Blocks[0].Category)
	require.Contains(t, PromptContentText(result), "SEARCH_RESULT")
	require.Contains(t, PromptContentText(result), "REFUSAL")
	require.Equal(t, 1, PromptContentMessageCount(result))
	require.Equal(t, 2, result.Filtered["tool_definition"].Count)
	require.Equal(t, 1, result.Filtered["reasoning_summary"].Count)
	require.Equal(t, 1, result.Filtered["encrypted"].Count)
	all := AnalyzePromptContent("responses", input, contentTestPolicyAll(), 30)
	require.Contains(t, PromptContentText(all), "SUMMARY")
	require.Contains(t, PromptContentText(all), "DEFINITION_TWO")
	require.NotContains(t, PromptContentText(all), "COMPACTION_SECRET")
	require.Equal(t, 1, PromptContentMessageCount(all), "only the assistant refusal consumes a dialogue message slot")
}

func TestPromptContentDocumentUnlimitedResponseMessages(t *testing.T) {
	messages := make([]any, 40)
	for index := range messages {
		messages[index] = map[string]any{"role": "assistant", "content": "answer"}
	}
	result := AnalyzePromptContent("responses", map[string]any{"output": messages}, RecommendedPromptRecordingPolicy(), -1)
	require.Len(t, result.Blocks, 40)
	require.Equal(t, 40, PromptContentMessageCount(result))
	require.False(t, result.Truncated)
}

func TestPromptContentDocumentImageGenerationResultBoundary(t *testing.T) {
	input := contentTestDecode(t, `{"output":[{"type":"image_generation_call","id":"image1","result":"BUILTIN_BASE64_IMAGE","revised_prompt":"draw a tree"},{"type":"function_call","name":"business","arguments":{"result":"BUSINESS_CALL_RESULT"}},{"type":"function_call_output","call_id":"business1","result":"BUSINESS_RESULT"}]}`)
	result := AnalyzePromptContent("responses", input, contentTestPolicyAll(), 30)
	text := PromptContentText(result)
	require.NotContains(t, text, "BUILTIN_BASE64_IMAGE")
	require.Contains(t, text, "draw a tree")
	require.Contains(t, text, "BUSINESS_CALL_RESULT")
	require.Contains(t, text, "BUSINESS_RESULT")
	require.Equal(t, 1, result.Filtered["media"].Count)
	require.Equal(t, len("BUILTIN_BASE64_IMAGE"), result.Filtered["media"].Bytes)
}

func TestPromptContentTextBoundsIndentationDepth(t *testing.T) {
	var nested any = "readable leaf"
	for range 700 {
		nested = map[string]any{"nested": nested}
	}
	compact, err := json.Marshal(nested)
	require.NoError(t, err)
	text := PromptContentText(PromptContentDocument{Blocks: []PromptContentBlock{{Category: "tool_result", Data: nested}}})
	require.Equal(t, string(compact), text, "deep JSON remains readable without quadratic indentation")

	shallow := map[string]any{"text": strings.Repeat(`{[\"quoted\"]}`, 100), "result": "readable"}
	pretty, err := json.MarshalIndent(shallow, "", "  ")
	require.NoError(t, err)
	text = PromptContentText(PromptContentDocument{Blocks: []PromptContentBlock{{Category: "tool_result", Data: shallow}}})
	require.Equal(t, string(pretty), text, "brackets and escaped quotes inside strings do not count toward nesting")
}

func TestPromptContentTextKeepsReadableMarkupAndValidJSON(t *testing.T) {
	text := `<skills_instructions>Read "notes" & C:\project</skills_instructions>`
	for _, depth := range []int{0, 40} {
		var data any = map[string]any{"content": text}
		for range depth {
			data = map[string]any{"nested": data}
		}
		rendered := PromptContentText(PromptContentDocument{Blocks: []PromptContentBlock{{Category: "tool_result", Data: data}}})
		require.Contains(t, rendered, "<skills_instructions>")
		require.Contains(t, rendered, "</skills_instructions>")
		require.NotContains(t, rendered, `\u003c`)
		require.NotContains(t, rendered, `\u0026`)
		var decoded any
		require.NoError(t, json.Unmarshal([]byte(rendered), &decoded))
		require.Equal(t, data, decoded, "quotes and backslashes must remain valid JSON")
	}
}
