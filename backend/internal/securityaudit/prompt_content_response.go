package securityaudit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ExtractRecordedResponse applies the request's policy snapshot, or the
// recommended content policy for direct callers without a captured snapshot.
func ExtractRecordedResponse(req Request, body []byte, truncated bool) (PromptResponse, bool) {
	if !ShouldRecordPromptResponse(req) {
		return PromptResponse{}, false
	}
	payload, recognized, incomplete := recordedResponsePayload(body)
	if !recognized && !truncated && !incomplete {
		return PromptResponse{}, false
	}
	// Response streams contain output items, not the request's conversation
	// history. The encoded response limit below bounds their retained size.
	document := AnalyzePromptContent(req.Protocol, payload, promptRecordingRequestPolicy(req), -1)
	document.Truncated = document.Truncated || truncated || incomplete
	document = limitRecordedResponseDocument(document, PromptResponseTextLimit)
	encoded, err := json.Marshal(document)
	if err != nil {
		return PromptResponse{}, false
	}
	text := PromptContentText(document)
	return PromptResponse{Text: text, Document: string(encoded), Length: utf8.RuneCountInString(text), Truncated: document.Truncated, CapturedAt: time.Now().UTC()}, true
}

// ShouldRecordPromptResponse lets the transport honor the same immutable
// settings selected when the request entered prompt recording.
func ShouldRecordPromptResponse(req Request) bool {
	return req.recordingConfig == nil || req.recordingConfig.Enabled && req.recordingConfig.ResponseEnabled
}

func limitRecordedResponseDocument(document PromptContentDocument, limit int) PromptContentDocument {
	fits := func(candidate PromptContentDocument) bool {
		encoded, err := json.Marshal(candidate)
		return err == nil && len(encoded) <= limit && len(PromptContentText(candidate)) <= limit
	}
	if fits(document) {
		return document
	}
	document.Truncated = true
	original := document.Blocks
	// Find the largest retained prefix without repeatedly removing and encoding
	// one block at a time for streams with many small output blocks.
	low, high := 0, len(original)
	for low < high {
		middle := (low + high + 1) / 2
		document.Blocks = original[:middle]
		if fits(document) {
			low = middle
		} else {
			high = middle - 1
		}
	}
	document.Blocks = append([]PromptContentBlock{}, original[:low]...)
	if low == len(original) {
		return document
	}
	last := original[low]
	text := PromptContentText(PromptContentDocument{Blocks: []PromptContentBlock{last}})
	last.Data = nil
	// A partially retained JSON result is explicitly a readable text excerpt;
	// the canonical document remains valid and carries its truncated marker.
	low, high = 0, len(text)
	base := len(document.Blocks)
	document.Blocks = append(document.Blocks, last)
	for low < high {
		middle := (low + high + 1) / 2
		document.Blocks[base].Text, _ = truncateUTF8(text, middle)
		if fits(document) {
			low = middle
		} else {
			high = middle - 1
		}
	}
	document.Blocks[base].Text, _ = truncateUTF8(text, low)
	if document.Blocks[base].Text == "" {
		document.Blocks = document.Blocks[:base]
	}
	return document
}

// Normalize JSON response envelopes to an assistant message collection consumed
// by the shared classifier. Nested tool arguments remain business JSON.
func normalizeRecordedResponse(payload any) any {
	root, ok := payload.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	if response, ok := root["response"].(map[string]any); ok {
		return normalizeRecordedResponse(response)
	}
	normalized := make(map[string]any, len(root))
	for key, value := range root {
		normalized[key] = value
	}
	if choices, ok := root["choices"].([]any); ok {
		messages := make([]any, 0, len(choices))
		for _, raw := range choices {
			choice, _ := raw.(map[string]any)
			if message, ok := choice["message"].(map[string]any); ok {
				messages = append(messages, assistantResponseMessage(message))
				continue
			}
			if delta, ok := choice["delta"].(map[string]any); ok {
				messages = append(messages, assistantResponseMessage(delta))
				continue
			}
			if text, ok := choice["text"].(string); ok {
				messages = append(messages, map[string]any{"role": "assistant", "content": text})
			}
		}
		delete(normalized, "choices")
		normalized["messages"] = messages
		return normalized
	}
	if candidates, ok := root["candidates"].([]any); ok {
		contents := make([]any, 0, len(candidates))
		for _, raw := range candidates {
			candidate, _ := raw.(map[string]any)
			if content, ok := candidate["content"].(map[string]any); ok {
				contents = append(contents, assistantResponseMessage(content))
			}
		}
		delete(normalized, "candidates")
		normalized["contents"] = contents
		return normalized
	}
	if _, ok := root["output"].([]any); ok {
		return normalized
	}
	if content, exists := root["content"]; exists {
		delete(normalized, "content")
		// A Claude response's type is "message". Keep it as envelope
		// metadata rather than letting it hide the normalized messages.
		delete(normalized, "type")
		delete(normalized, "role")
		normalized["messages"] = []any{map[string]any{"role": "assistant", "content": content}}
		return normalized
	}
	if _, ok := root["output_text"].(string); ok {
		return normalized
	}
	return root
}

func assistantResponseMessage(message map[string]any) map[string]any {
	copy := make(map[string]any, len(message)+1)
	for key, value := range message {
		copy[key] = value
	}
	copy["role"] = "assistant"
	return copy
}

type recordedStreamItem struct {
	item  map[string]any
	final bool
}

func recordedResponsePayload(body []byte) (any, bool, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return map[string]any{}, false, false
	}
	if !bytes.HasPrefix(trimmed, []byte("data:")) && !bytes.Contains(trimmed, []byte("\ndata:")) {
		document, err := decodePromptDocument(trimmed)
		if err == nil {
			return normalizeRecordedResponse(document), true, false
		}
		if recovered, err := decodePromptDocument(completeResponseJSONPrefix(trimmed)); err == nil {
			return normalizeRecordedResponse(recovered), true, true
		}
		return map[string]any{}, false, true
	}
	items := map[string]*recordedStreamItem{}
	itemKeys := map[string]string{}
	order := []string{}
	metadata := map[string]any{}
	get := func(key string, initial map[string]any) *recordedStreamItem {
		if existing := items[key]; existing != nil {
			return existing
		}
		item := &recordedStreamItem{item: initial}
		items[key] = item
		order = append(order, key)
		return item
	}
	appendField := func(item map[string]any, key string, value any) {
		if text, ok := value.(string); ok {
			old, _ := item[key].(string)
			item[key] = old + text
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	scanner.Buffer(make([]byte, 64*1024), len(trimmed)+1)
	recognized, incomplete := false, false
	var finalPayload any
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		decoded, err := decodePromptDocument([]byte(data))
		if err != nil {
			incomplete = true
			decoded, err = decodePromptDocument(completeResponseJSONPrefix([]byte(data)))
			if err != nil {
				continue
			}
		}
		root, ok := decoded.(map[string]any)
		if !ok {
			continue
		}
		recognized = true
		kind, _ := root["type"].(string)
		key := fmt.Sprint(root["output_index"])
		if root["output_index"] == nil {
			if mapped, exists := itemKeys[contentString(root, "item_id")]; exists {
				key = mapped
			}
		}
		for _, field := range []string{"usage", "usageMetadata", "model", "id", "system_fingerprint"} {
			if value, exists := root[field]; exists {
				metadata[field] = value
			}
		}
		switch kind {
		case "response.completed", "response.done":
			if response, ok := root["response"].(map[string]any); ok {
				if _, exists := response["output"]; exists {
					finalPayload = normalizeRecordedResponse(response)
				}
			}
		case "response.output_item.added", "response.output_item.done":
			item, ok := root["item"].(map[string]any)
			if !ok {
				continue
			}
			entry := get(key, item)
			if id := contentString(item, "id"); id != "" {
				itemKeys[id] = key
			}
			if kind == "response.output_item.done" {
				entry.item = item
				entry.final = true
				// The final reasoning item already includes the collected
				// summary/reasoning deltas. Keep a single copy.
				delete(items, key+":summary_text")
				delete(items, key+":thinking")
			}
		case "response.output_text.delta", "response.refusal.delta":
			entry := get(key, map[string]any{"role": "assistant"})
			if entry.final {
				continue
			}
			field := "content"
			if kind == "response.refusal.delta" {
				field = "content"
			}
			appendField(entry.item, field, root["delta"])
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			categoryKind := "thinking"
			field := "thinking"
			if kind == "response.reasoning_summary_text.delta" {
				categoryKind = "reasoning"
				field = "summary_text"
			}
			if original := items[key]; original != nil && original.final {
				continue
			}
			entry := get(key+":"+field, map[string]any{"type": categoryKind})
			appendField(entry.item, field, root["delta"])
		case "response.function_call_arguments.delta":
			entry := get(key, map[string]any{"type": "function_call", "call_id": root["item_id"]})
			if !entry.final {
				appendField(entry.item, "arguments", root["delta"])
			}
		case "response.custom_tool_call_input.delta":
			entry := get(key, map[string]any{"type": "custom_tool_call", "call_id": root["item_id"]})
			if !entry.final {
				appendField(entry.item, "input", root["delta"])
			}
		case "content_block_start":
			block, ok := root["content_block"].(map[string]any)
			if ok {
				get("claude:"+fmt.Sprint(root["index"]), block)
			}
		case "content_block_delta":
			delta, _ := root["delta"].(map[string]any)
			key = "claude:" + fmt.Sprint(root["index"])
			deltaType, _ := delta["type"].(string)
			switch deltaType {
			case "thinking_delta":
				entry := get(key, map[string]any{"type": "thinking"})
				appendField(entry.item, "thinking", delta["thinking"])
			case "input_json_delta":
				entry := get(key, map[string]any{"type": "tool_use"})
				appendField(entry.item, "arguments", delta["partial_json"])
			case "signature_delta": // Never accumulate opaque signature deltas.
			default:
				entry := get(key, map[string]any{"type": "text"})
				appendField(entry.item, "text", delta["text"])
			}
		case "message_start":
			if message, ok := root["message"].(map[string]any); ok {
				for _, field := range []string{"id", "model", "usage"} {
					if value, exists := message[field]; exists {
						metadata[field] = value
					}
				}
			}
		case "message_delta":
			if delta, ok := root["delta"].(map[string]any); ok {
				for _, field := range []string{"stop_reason", "stop_sequence"} {
					if value, exists := delta[field]; exists {
						metadata[field] = value
					}
				}
			}
		default:
			if choices, ok := root["choices"].([]any); ok {
				for _, raw := range choices {
					choice, _ := raw.(map[string]any)
					delta, _ := choice["delta"].(map[string]any)
					if delta == nil {
						continue
					}
					prefix := "chat:" + fmt.Sprint(choice["index"])
					entry := get(prefix, map[string]any{"role": "assistant"})
					appendField(entry.item, "content", delta["content"])
					appendField(entry.item, "content", delta["refusal"])
					appendField(entry.item, "reasoning_content", delta["reasoning_content"])
					if calls, ok := delta["tool_calls"].([]any); ok {
						for _, rawCall := range calls {
							call, _ := rawCall.(map[string]any)
							tool := get(prefix+":tool:"+fmt.Sprint(call["index"]), map[string]any{"type": "function_call"})
							if id, ok := call["id"].(string); ok {
								tool.item["call_id"] = id
							}
							function, _ := call["function"].(map[string]any)
							appendField(tool.item, "name", function["name"])
							appendField(tool.item, "arguments", function["arguments"])
						}
					}
					if function, ok := delta["function_call"].(map[string]any); ok {
						tool := get(prefix+":function", map[string]any{"type": "function_call"})
						appendField(tool.item, "name", function["name"])
						appendField(tool.item, "arguments", function["arguments"])
					}
				}
			} else if candidates, ok := root["candidates"].([]any); ok {
				// Streaming parts belong to a candidate's single assistant
				// message, regardless of the number of network frames.
				for index, raw := range candidates {
					candidate, _ := raw.(map[string]any)
					candidateIndex := candidate["index"]
					if candidateIndex == nil {
						candidateIndex = index
					}
					entry := get("gemini:"+fmt.Sprint(candidateIndex), map[string]any{"role": "assistant", "parts": []any{}})
					content, _ := candidate["content"].(map[string]any)
					parts, _ := entry.item["parts"].([]any)
					for _, rawPart := range contentSlice(content["parts"]) {
						part, _ := rawPart.(map[string]any)
						text, isText := part["text"].(string)
						if isText && len(parts) > 0 {
							last, _ := parts[len(parts)-1].(map[string]any)
							if previous, ok := last["text"].(string); ok && last["thought"] == part["thought"] && len(part) == 1 {
								last["text"] = previous + text
								continue
							}
						}
						parts = append(parts, part)
					}
					entry.item["parts"] = parts
				}
			}
		}
	}
	if finalPayload != nil {
		return finalPayload, recognized, incomplete || scanner.Err() != nil
	}
	output := make([]any, 0, len(order))
	for _, key := range order {
		if item := items[key]; item != nil {
			output = append(output, item.item)
		}
	}
	metadata["output"] = output
	return metadata, recognized, incomplete || scanner.Err() != nil
}
