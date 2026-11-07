package securityaudit

import (
	"encoding/json"
	"strings"
)

// extractPromptRecordSegments reads historical request_body documents. New
// records use PromptContentDocument. This fallback does not filter content and
// must not calculate the original request's audit/correlation identity.
func extractPromptRecordSegments(protocol string, document any) []promptSegment {
	if legacyRecordMediaProtocol(protocol) {
		return extractProtocolSegments(protocol, document)
	}
	root, ok := document.(map[string]any)
	if !ok {
		return extractProtocolSegments(protocol, document)
	}
	var result []promptSegment
	for _, key := range []string{"messages", "input", "contents", "content"} {
		result = appendPromptRecordSegments(result, root[key], "user")
	}
	for _, key := range []string{"output", "output_text"} {
		result = appendPromptRecordSegments(result, root[key], "assistant")
	}
	if response, ok := root["response"].(map[string]any); ok {
		result = append(result, extractPromptRecordSegments(protocol, response)...)
	}
	if requests, ok := root["requests"].([]any); ok {
		for _, request := range requests {
			result = append(result, extractPromptRecordSegments(protocol, request)...)
		}
	}
	if len(result) == 0 {
		// Preserve the existing media prompt and other protocol-specific shapes.
		return extractProtocolSegments(protocol, document)
	}
	return result
}

func appendPromptRecordSegments(result []promptSegment, value any, role string) []promptSegment {
	switch typed := value.(type) {
	case string:
		return appendPromptRecordText(result, typed, role)
	case []any:
		for _, item := range typed {
			result = appendPromptRecordSegments(result, item, role)
		}
	case map[string]any:
		if itemRole, ok := typed["role"].(string); ok {
			role = strings.ToLower(strings.TrimSpace(itemRole))
		}
		kind := strings.ToLower(stringValue(typed["type"]))
		if kind == "reasoning" || kind == "thinking" || kind == "thought" || kind == "refusal" {
			role = "assistant"
		}
		if legacyRecordToolResultKind(kind) {
			for _, key := range []string{"output", "content", "result"} {
				if data, exists := typed[key]; exists {
					result = appendPromptRecordData(result, data, "tool")
				}
			}
			return result
		}
		if legacyRecordToolCallKind(kind) {
			return appendPromptRecordCall(result, typed, "assistant")
		}
		for _, key := range []string{"text", "refusal", "thinking", "reasoning_content"} {
			if text, ok := typed[key].(string); ok {
				result = appendPromptRecordText(result, text, role)
			}
		}
		for _, key := range []string{"content", "parts", "summary"} {
			if data, exists := typed[key]; exists {
				if role == "tool" || role == "function" {
					result = appendPromptRecordData(result, data, role)
				} else {
					result = appendPromptRecordSegments(result, data, role)
				}
			}
		}
		if calls, ok := typed["tool_calls"].([]any); ok {
			for _, call := range calls {
				if object, ok := call.(map[string]any); ok {
					for _, key := range []string{"function", "custom"} {
						if call, ok := object[key].(map[string]any); ok {
							result = appendPromptRecordCall(result, call, "assistant")
						}
					}
				}
			}
		}
		for _, key := range []string{"function_call", "functionCall"} {
			if call, ok := typed[key].(map[string]any); ok {
				result = appendPromptRecordCall(result, call, "assistant")
			}
		}
		for _, key := range []string{"functionResponse", "function_response"} {
			if response, exists := typed[key]; exists {
				result = appendPromptRecordData(result, response, "tool")
			}
		}
	}
	return result
}

func legacyRecordMediaProtocol(protocol string) bool {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "openai_images", "grok_media", "media", "images":
		return true
	default:
		return false
	}
}

func legacyRecordToolCallKind(kind string) bool {
	return kind == "tool_use" || strings.HasSuffix(kind, "_tool_use") || kind == "function" ||
		strings.HasSuffix(kind, "_call") || legacyRecordToolResultKind(kind)
}

func legacyRecordToolResultKind(kind string) bool {
	return kind == "tool_result" || strings.HasSuffix(kind, "_tool_result") || strings.HasSuffix(kind, "_call_output")
}

func appendPromptRecordCall(result []promptSegment, call map[string]any, role string) []promptSegment {
	payload := make(map[string]any)
	for key, value := range call {
		switch key {
		case "type", "id", "call_id", "status":
			continue
		}
		payload[key] = value
	}
	if len(payload) == 0 {
		return result
	}
	return appendPromptRecordData(result, payload, role)
}

func appendPromptRecordData(result []promptSegment, data any, role string) []promptSegment {
	if text, ok := data.(string); ok {
		return appendPromptRecordText(result, text, role)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return result
	}
	return appendPromptRecordText(result, string(encoded), role)
}

func appendPromptRecordText(result []promptSegment, text, role string) []promptSegment {
	if strings.TrimSpace(text) == "" {
		return result
	}
	return append(result, promptSegment{text: text, user: role == "user", role: role})
}
