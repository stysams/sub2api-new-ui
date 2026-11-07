package securityaudit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// PromptContentBlock is one independently filterable, readable portion of a
// protocol message. Data holds application JSON without interpreting its keys.
type PromptContentBlock struct {
	Category string `json:"category"`
	Role     string `json:"role,omitempty"`
	Path     string `json:"path,omitempty"`
	Text     string `json:"text,omitempty"`
	Data     any    `json:"data,omitempty"`
	CallID   string `json:"call_id,omitempty"`
	Name     string `json:"name,omitempty"`
	Group    string `json:"group,omitempty"`
}

type PromptContentCount struct {
	Count int `json:"count"`
	Bytes int `json:"bytes"`
}

type PromptContentDocument struct {
	SchemaVersion int                           `json:"schema_version"`
	Blocks        []PromptContentBlock          `json:"blocks"`
	Filtered      map[string]PromptContentCount `json:"filtered"`
	Truncated     bool                          `json:"truncated,omitempty"`
}

const (
	promptContentSchemaVersion = 2
	// Used when no max_messages setting exists or its value is out of range.
	promptRecordDefaultMaxMessages = 30
)

var contentMediaRootKeys = []string{
	"prompt", "input_prompt", "text_prompt", "description", "query", "lyrics",
	"negative_prompt", "positive_prompt", "gpt_description_prompt", "prompt_en",
	"final_prompt", "final_zh_prompt", "orig_prompt", "actual_prompt", "image_prompt",
	"input", "request", "image", "images",
}

type promptContentAnalyzer struct {
	policy         PromptRecordingPolicy
	document       PromptContentDocument
	groups         []string
	groupSet       map[string]bool
	dialogueGroups map[string]bool
	callNames      map[string]string
	identities     map[string]bool
}

// AnalyzePromptContent recognizes protocol positions before applying policy.
// In particular, tool arguments and result JSON are never recursively treated
// as messages, metadata, thinking blocks, or files.
func AnalyzePromptContent(protocol string, document any, policy PromptRecordingPolicy, maxMessages int) PromptContentDocument {
	if maxMessages != -1 && (maxMessages < 1 || maxMessages > 999) {
		maxMessages = promptRecordDefaultMaxMessages
	}
	a := &promptContentAnalyzer{
		policy: policy,
		document: PromptContentDocument{
			SchemaVersion: promptContentSchemaVersion,
			Blocks:        make([]PromptContentBlock, 0),
			Filtered:      make(map[string]PromptContentCount),
		},
		groupSet: make(map[string]bool), dialogueGroups: make(map[string]bool), callNames: make(map[string]string),
		identities: contentAgentIdentities(document),
	}
	switch typed := document.(type) {
	case PromptContentDocument:
		a.readRetainedDocument(typed)
	case map[string]any:
		if contentRetainedDocument(typed) {
			a.readRetainedMap(typed)
		} else {
			a.readDocument(typed, "", "")
		}
	default:
		a.readDocument(document, "", "")
	}
	if maxMessages != -1 {
		a.limitMessages(maxMessages)
	}
	return a.document
}

// PromptContentText follows the original block order and renders tool JSON as
// readable JSON. It never reconstructs filtered input.
func PromptContentText(document PromptContentDocument) string {
	parts := make([]string, 0, len(document.Blocks))
	for _, block := range document.Blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
		if block.Data != nil {
			var compact bytes.Buffer
			encoder := json.NewEncoder(&compact)
			encoder.SetEscapeHTML(false)
			err := encoder.Encode(block.Data)
			if err == nil {
				encoded := bytes.TrimSuffix(compact.Bytes(), []byte{'\n'})
				if !contentJSONExceedsDepth(encoded, 32) {
					var pretty bytes.Buffer
					if json.Indent(&pretty, encoded, "", "  ") == nil {
						encoded = pretty.Bytes()
					}
				}
				parts = append(parts, string(encoded))
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// Indentation grows quadratically with nesting depth. Scan compact, encoded
// JSON before formatting it, ignoring escaped quotes and brackets in strings.
func contentJSONExceedsDepth(encoded []byte, limit int) bool {
	depth := 0
	quoted, escaped := false, false
	for _, char := range encoded {
		if quoted {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				quoted = false
			}
			continue
		}
		switch char {
		case '"':
			quoted = true
		case '{', '[':
			depth++
			if depth > limit {
				return true
			}
		case '}', ']':
			depth--
		}
	}
	return false
}

func PromptContentMessageCount(document PromptContentDocument) int {
	groups := make(map[string]bool)
	for _, block := range document.Blocks {
		if block.Group != "" && contentDialogueBlock(block.Category, block.Text, block.Data) {
			groups[block.Group] = true
		}
	}
	return len(groups)
}

func (a *promptContentAnalyzer) readRetainedDocument(document PromptContentDocument) {
	for category, count := range document.Filtered {
		a.document.Filtered[category] = count
	}
	a.document.Truncated = document.Truncated
	for _, block := range document.Blocks {
		a.registerGroup(block.Group)
		a.addBlock(block.Category, block.Role, block.Path, block.Text, block.Data, block.CallID, block.Name, block.Group)
	}
}

func (a *promptContentAnalyzer) readRetainedMap(root map[string]any) {
	if stats, ok := root["filtered"].(map[string]any); ok {
		for category, raw := range stats {
			if count, ok := raw.(map[string]any); ok {
				a.document.Filtered[category] = PromptContentCount{Count: contentInt(count["count"]), Bytes: contentInt(count["bytes"])}
			}
		}
	}
	a.document.Truncated, _ = root["truncated"].(bool)
	for _, raw := range contentSlice(root["blocks"]) {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		group := contentString(block, "group")
		a.registerGroup(group)
		a.addBlock(contentString(block, "category"), contentString(block, "role"), contentString(block, "path"), contentString(block, "text"), block["data"], contentString(block, "call_id"), contentString(block, "name"), group)
	}
}

func (a *promptContentAnalyzer) readDocument(value any, path, role string) {
	switch typed := value.(type) {
	case string:
		a.addText(role, typed, path, "")
	case []any:
		for index, item := range typed {
			a.readDocument(item, contentIndex(path, index), role)
		}
	case map[string]any:
		if contentMessageObject(typed) {
			group := ""
			if !contentAuxiliaryItem(typed) {
				group = a.messageGroup(path)
			}
			a.readMessage(typed, path, role, group)
			return
		}
		a.readProtocolObject(typed, path, role)
	}
}

func (a *promptContentAnalyzer) readProtocolObject(root map[string]any, path, role string) {
	// Instruction fields precede conversation turns in the protocol's logical
	// order. Maps have no source ordering after JSON decoding.
	for _, key := range []string{"instructions", "system", "systemInstruction", "system_instruction"} {
		if value, exists := root[key]; exists {
			source := "system"
			if key == "instructions" {
				source = "instructions"
			}
			a.readContent(value, contentChild(path, key), source, "")
		}
	}
	handled := false
	for _, key := range []string{"messages", "input", "contents"} {
		if value, exists := root[key]; exists {
			a.readMessages(value, contentChild(path, key), role)
			handled = true
		}
	}
	if value, exists := root["output"]; exists {
		a.readMessages(value, contentChild(path, "output"), "assistant")
		handled = true
	} else if text, ok := root["output_text"].(string); ok {
		a.addText("assistant", text, contentChild(path, "output_text"), a.messageGroup(contentChild(path, "output_text")))
		handled = true
	}
	for _, key := range []string{"choices", "candidates"} {
		if value, exists := root[key]; exists {
			for index, raw := range contentSlice(value) {
				item, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				itemPath := contentIndex(contentChild(path, key), index)
				group := a.messageGroup(itemPath)
				if message, exists := item["message"]; exists {
					a.readMessage(message, contentChild(itemPath, "message"), "assistant", group)
				} else if delta, exists := item["delta"]; exists {
					a.readMessage(delta, contentChild(itemPath, "delta"), "assistant", group)
				} else if content, exists := item["content"]; exists {
					a.readMessage(content, contentChild(itemPath, "content"), "assistant", group)
				} else if text, ok := item["text"].(string); ok {
					a.addText("assistant", text, contentChild(itemPath, "text"), group)
				}
				a.readAuxiliary(item, itemPath, "assistant", group)
			}
			handled = true
		}
	}
	for _, key := range []string{"request", "response"} {
		if value, exists := root[key]; exists {
			a.readDocument(value, contentChild(path, key), role)
			handled = true
		}
	}
	if requests, exists := root["requests"]; exists {
		for index, raw := range contentSlice(requests) {
			a.readDocument(raw, contentIndex(contentChild(path, "requests"), index), role)
		}
		handled = true
	}
	for _, key := range []string{"tools", "functions"} {
		if value, exists := root[key]; exists {
			a.addBlock("tool_definition", "", contentChild(path, key), "", a.cleanProtocolValue(value), "", "", "")
		}
	}
	for _, key := range contentMediaRootKeys {
		if key == "prompt" || key == "input" || key == "request" || key == "image" || key == "images" {
			continue
		}
		if text, ok := root[key].(string); ok {
			a.addText("user", text, contentChild(path, key), a.messageGroup(path))
		}
	}
	for _, key := range []string{"naturalprompt", "natural_prompt"} {
		if text, ok := root[key].(string); ok {
			a.addText("user", text, contentChild(path, key), a.messageGroup(path))
		}
	}
	if !handled {
		if content, exists := root["content"]; exists {
			a.readContent(content, contentChild(path, "content"), role, a.messageGroup(path))
		} else if text, ok := root["text"].(string); ok {
			a.addText(role, text, contentChild(path, "text"), a.messageGroup(path))
		} else if prompt, ok := root["prompt"].(string); ok {
			a.addText("user", prompt, contentChild(path, "prompt"), a.messageGroup(path))
		}
	}
	a.readAuxiliary(root, path, role, "")
	a.readRootMetadata(root, path)
}

func (a *promptContentAnalyzer) readMessages(value any, path, role string) {
	switch typed := value.(type) {
	case []any:
		for index, item := range typed {
			itemPath := contentIndex(path, index)
			groupPath := itemPath
			if contentNonMessageItem(item) {
				groupPath = ""
			} else if contentAuxiliaryItem(item) {
				groupPath = ""
				for next := index + 1; next < len(typed); next++ {
					if !contentAuxiliaryItem(typed[next]) && !contentNonMessageItem(typed[next]) {
						groupPath = contentIndex(path, next)
						break
					}
				}
				if groupPath == "" {
					for previous := index - 1; previous >= 0; previous-- {
						if !contentAuxiliaryItem(typed[previous]) && !contentNonMessageItem(typed[previous]) {
							groupPath = contentIndex(path, previous)
							break
						}
					}
				}
			}
			group := ""
			if groupPath != "" {
				group = a.messageGroup(groupPath)
			}
			a.readMessage(item, itemPath, role, group)
		}
	case map[string]any:
		group := ""
		if !contentAuxiliaryItem(typed) {
			group = a.messageGroup(path)
		}
		a.readMessage(typed, path, role, group)
	case string:
		a.addText(role, typed, path, a.messageGroup(path))
	}
}

func (a *promptContentAnalyzer) readMessage(raw any, path, inheritedRole, group string) {
	item, ok := raw.(map[string]any)
	if !ok {
		a.readContent(raw, path, inheritedRole, group)
		return
	}
	role := contentString(item, "role")
	if role == "" {
		role = inheritedRole
	}
	if role == "tool" || role == "function" {
		a.addToolResult(item, path, group)
		return
	}
	textRole := role
	if role == "assistant" || role == "model" {
		channel := strings.ToLower(contentString(item, "channel"))
		if channel == "analysis" || channel == "thinking" || channel == "reasoning" {
			textRole = "thinking"
		}
	}
	kind := strings.ToLower(contentString(item, "type"))
	if kind != "" && kind != "message" {
		a.readPart(item, path, textRole, group)
		return
	}
	if content, exists := item["content"]; exists {
		a.readContent(content, contentChild(path, "content"), textRole, group)
	}
	if parts, exists := item["parts"]; exists {
		a.readContent(parts, contentChild(path, "parts"), textRole, group)
	}
	if text, ok := item["text"].(string); ok {
		a.addText(textRole, text, contentChild(path, "text"), group)
	}
	if refusal, ok := item["refusal"].(string); ok && refusal != "" {
		a.addBlock("assistant", "assistant", contentChild(path, "refusal"), refusal, nil, "", "", group)
	}
	if text, ok := item["reasoning_content"].(string); ok && text != "" {
		a.addBlock("thinking", role, contentChild(path, "reasoning_content"), text, nil, "", "", group)
	}
	if audio, ok := item["audio"].(map[string]any); ok {
		if transcript, ok := audio["transcript"].(string); ok {
			a.addText(role, transcript, contentChild(path, "audio/transcript"), group)
		}
		if data, exists := audio["data"]; exists {
			a.filtered("media", data)
		}
		for _, key := range []string{"id", "expires_at"} {
			if raw, exists := audio[key]; exists {
				a.addBlock("metadata", role, contentChild(path, "audio/"+key), "", raw, "", "", group)
			}
		}
	}
	for index, raw := range contentSlice(item["tool_calls"]) {
		if call, ok := raw.(map[string]any); ok {
			a.addToolCall(call, contentIndex(contentChild(path, "tool_calls"), index), group)
		}
	}
	if call, ok := item["function_call"].(map[string]any); ok {
		a.addToolCall(call, contentChild(path, "function_call"), group)
	}
	a.readAuxiliary(item, path, role, group)
}

func (a *promptContentAnalyzer) readContent(value any, path, role, group string) {
	switch typed := value.(type) {
	case string:
		a.addText(role, typed, path, group)
	case []any:
		for index, part := range typed {
			a.readPart(part, contentIndex(path, index), role, group)
		}
	case map[string]any:
		if parts, exists := typed["parts"]; exists {
			a.readContent(parts, contentChild(path, "parts"), role, group)
			a.readAuxiliary(typed, path, role, group)
		} else {
			a.readPart(typed, path, role, group)
		}
	}
}

func (a *promptContentAnalyzer) readPart(raw any, path, role, group string) {
	part, ok := raw.(map[string]any)
	if !ok {
		if text, ok := raw.(string); ok {
			a.addText(role, text, path, group)
		}
		return
	}
	kind := strings.ToLower(contentString(part, "type"))
	switch {
	case kind == "additional_tools":
		a.addBlock("tool_definition", role, path, "", a.cleanProtocolObject(part), "", "", "")
	case contentStateKind(kind):
		a.addBlock("metadata", role, path, "", a.cleanProtocolObject(part), "", "", "")
	case contentMediaKind(kind), contentMediaPart(part):
		a.filtered("media", part)
	case kind == "redacted_thinking":
		a.filtered("encrypted", part)
	case contentToolResultKind(kind), part["functionResponse"] != nil, part["function_response"] != nil:
		a.addToolResult(part, path, group)
	case contentToolCallKind(kind), part["functionCall"] != nil, part["function_call"] != nil:
		a.addToolCall(part, path, group)
	case kind == "reasoning", kind == "thinking", kind == "compaction_summary", part["thought"] == true:
		a.addReasoning(part, path, group)
	case kind == "refusal":
		a.addBlock("assistant", "assistant", path, contentString(part, "refusal"), nil, "", "", group)
		a.readAuxiliary(part, path, role, group)
	default:
		if text, ok := part["text"].(string); ok {
			if kind == "summary_text" {
				a.addBlock("reasoning_summary", "assistant", path, text, nil, "", "", group)
			} else {
				a.addText(role, text, contentChild(path, "text"), group)
			}
			a.readAuxiliary(part, path, role, group)
		} else if _, exists := part["content"]; exists && (kind == "message" || part["role"] != nil) {
			a.readMessage(part, path, role, group)
		} else {
			// An unrecognized content block is still readable data. Only known
			// protocol fields at this boundary are removed.
			data := a.cleanProtocolObject(part)
			if len(data) > 0 {
				a.addBlock(contentRoleCategory(role), role, path, "", data, "", "", group)
			}
		}
	}
}

func (a *promptContentAnalyzer) addToolCall(item map[string]any, path, group string) {
	callID := contentFirstString(item, "call_id", "tool_call_id", "tool_use_id", "id")
	payload := item
	for _, key := range []string{"function", "custom", "functionCall", "function_call"} {
		if nested, ok := item[key].(map[string]any); ok {
			payload = nested
			break
		}
	}
	name := contentString(payload, "name")
	if callID == "" {
		callID = contentFirstString(payload, "call_id", "id")
	}
	if callID != "" {
		a.callNames[callID] = name
	}
	data := make(map[string]any)
	for key, value := range payload {
		switch key {
		case "type", "id", "call_id", "tool_call_id", "tool_use_id", "role", "status":
			continue
		case "encrypted_content", "signature", "thoughtSignature", "thought_signature", "reasoning_signature":
			a.filtered("encrypted", value)
		case "metadata", "client_metadata", "cache_control", "logprobs", "token_ids", "usage", "usageMetadata", "annotations":
			a.addBlock("metadata", "assistant", contentChild(path, key), "", value, "", "", group)
		case "outputs":
			if contentString(item, "type") == "code_interpreter_call" {
				data[key] = a.cleanBuiltinOutputs(value)
			} else {
				data[key] = contentClone(value)
			}
		case "result":
			if contentString(item, "type") == "image_generation_call" {
				a.filtered("media", value)
			} else {
				data[key] = contentClone(value)
			}
		default:
			// arguments/input/args are application objects, including keys
			// whose spelling happens to match a protocol field.
			data[key] = contentClone(value)
		}
	}
	if payload != nil && len(data) > 0 {
		a.addBlock("tool_call", "assistant", path, "", data, callID, name, group)
	}
	if payload != nil && contentNestedToolPayload(item) {
		a.readAuxiliary(item, path, "assistant", group)
	}
}

func (a *promptContentAnalyzer) addToolResult(item map[string]any, path, group string) {
	callID := contentFirstString(item, "call_id", "tool_call_id", "tool_use_id")
	payload := item
	for _, key := range []string{"functionResponse", "function_response"} {
		if nested, ok := item[key].(map[string]any); ok {
			payload = nested
			break
		}
	}
	if callID == "" {
		callID = contentFirstString(payload, "call_id", "id")
	}
	name := contentString(payload, "name")
	if name == "" {
		name = a.callNames[callID]
	}
	data := make(map[string]any)
	for key, value := range payload {
		switch key {
		case "type", "id", "call_id", "tool_call_id", "tool_use_id", "role", "status":
			continue
		case "encrypted_content", "signature", "thoughtSignature", "thought_signature", "reasoning_signature":
			a.filtered("encrypted", value)
		case "metadata", "client_metadata", "cache_control", "logprobs", "token_ids", "usage", "usageMetadata", "annotations":
			a.addBlock("metadata", "tool", contentChild(path, key), "", value, "", "", group)
		case "content":
			data[key] = a.cleanToolResultContent(value)
		case "output":
			if contentString(item, "type") == "computer_call_output" {
				if output, ok := value.(map[string]any); ok && (contentMediaPart(output) || contentString(output, "type") == "computer_screenshot") {
					a.filtered("media", output)
					continue
				}
			}
			data[key] = contentClone(value)
		default:
			data[key] = contentClone(value)
		}
	}
	a.addBlock("tool_result", "tool", path, "", data, callID, name, group)
	if contentNestedToolPayload(item) {
		a.readAuxiliary(item, path, "tool", group)
	}
}

// Only Claude/Chat content arrays at a tool-result protocol boundary contain
// protocol blocks. output/response/result objects are arbitrary application data.
func (a *promptContentAnalyzer) cleanToolResultContent(value any) any {
	parts, ok := value.([]any)
	if !ok {
		return contentClone(value)
	}
	clean := make([]any, 0, len(parts))
	for _, raw := range parts {
		part, ok := raw.(map[string]any)
		if !ok {
			clean = append(clean, contentClone(raw))
			continue
		}
		kind := strings.ToLower(contentString(part, "type"))
		if contentMediaKind(kind) || contentMediaPart(part) {
			a.filtered("media", part)
			continue
		}
		if contentTextKind(kind) || contentServerResultKind(kind) {
			value := a.cleanProtocolObject(part)
			if nested, ok := value["content"].(map[string]any); ok && (contentMediaKind(contentString(nested, "type")) || contentMediaPart(nested)) {
				a.filtered("media", nested)
				delete(value, "content")
			}
			clean = append(clean, value)
		} else {
			// A JSON result can itself contain a type field. An unknown or
			// thinking type here is business data, not model reasoning.
			clean = append(clean, contentClone(part))
		}
	}
	return clean
}

func (a *promptContentAnalyzer) cleanBuiltinOutputs(value any) any {
	clean := make([]any, 0)
	for _, raw := range contentSlice(value) {
		if part, ok := raw.(map[string]any); ok {
			if contentMediaKind(contentString(part, "type")) || contentMediaPart(part) {
				a.filtered("media", part)
				continue
			}
			clean = append(clean, a.cleanProtocolObject(part))
		} else {
			clean = append(clean, contentClone(raw))
		}
	}
	return clean
}

func (a *promptContentAnalyzer) addReasoning(item map[string]any, path, group string) {
	if text := contentFirstString(item, "thinking", "text", "reasoning_content", "content"); text != "" {
		a.addBlock("thinking", "assistant", path, text, nil, "", "", group)
	}
	if summary, exists := item["summary"]; exists {
		for index, raw := range contentSlice(summary) {
			text, _ := raw.(string)
			if part, ok := raw.(map[string]any); ok {
				text = contentString(part, "text")
				a.readAuxiliary(part, contentIndex(contentChild(path, "summary"), index), "assistant", group)
			}
			if text != "" {
				a.addBlock("reasoning_summary", "assistant", contentIndex(contentChild(path, "summary"), index), text, nil, "", "", group)
			}
		}
	}
	if text := contentString(item, "summary_text"); text != "" {
		a.addBlock("reasoning_summary", "assistant", contentChild(path, "summary_text"), text, nil, "", "", group)
	}
	a.readAuxiliary(item, path, "assistant", group)
}

func (a *promptContentAnalyzer) addText(role, text, path, group string) {
	if text == "" {
		return
	}
	category := contentRoleCategory(role)
	if role == "thinking" {
		role = "assistant"
	}
	if category == "system" || category == "developer" || category == "instructions" {
		a.addInstructionText(category, role, text, path, group)
		return
	}
	if category == "user" && len(a.identities) > 0 {
		for {
			start := len(text) - len(strings.TrimLeft(text, " \t\r\n"))
			partCategory, end := contentTaggedPrefix(text[start:])
			allowed := a.identities["codex"] && (partCategory == "project" || partCategory == "environment") || a.identities["claude"] && partCategory == "reminder"
			if end == 0 || !allowed {
				break
			}
			a.addBlock(partCategory, role, path, text[:start+end], nil, "", "", group)
			text = text[start+end:]
		}
	}
	if text != "" {
		a.addBlock(category, role, path, text, nil, "", "", group)
	}
}

// Only instruction sources are split at known headings/tags. A tag in a code
// fence, quote, tool output, or ordinary user message is left untouched.
func (a *promptContentAnalyzer) addInstructionText(category, role, text, path, group string) {
	remainingStart, offset := 0, 0
	fence := ""
	for offset < len(text) {
		end := strings.IndexByte(text[offset:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += offset + 1
		}
		line := strings.TrimSpace(text[offset:end])
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			marker := line[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			offset = end
			continue
		}
		partCategory, partEnd := "", 0
		if fence == "" && !strings.HasPrefix(line, ">") {
			leading := len(text[offset:end]) - len(strings.TrimLeft(text[offset:end], " \t"))
			partCategory, partEnd = contentTaggedPrefix(text[offset+leading:])
			if partEnd > 0 {
				partEnd += offset + leading
			} else if strings.HasPrefix(line, "You are Codex") || strings.HasPrefix(line, "You are Claude Code") {
				partCategory, partEnd = "agent", end
			} else if contentSkillsHeading(line) {
				partCategory, partEnd = "skills", contentSectionEnd(text, offset, end)
			}
		}
		if partEnd > 0 {
			if offset > remainingStart {
				a.addBlock(category, role, path, text[remainingStart:offset], nil, "", "", group)
			}
			a.addBlock(partCategory, role, path, text[offset:partEnd], nil, "", "", group)
			remainingStart, offset = partEnd, partEnd
		} else {
			offset = end
		}
	}
	if remainingStart < len(text) {
		a.addBlock(category, role, path, text[remainingStart:], nil, "", "", group)
	}
}

func contentTaggedPrefix(text string) (string, int) {
	for _, heading := range []string{"# AGENTS.md instructions", "# CLAUDE.md instructions"} {
		if strings.HasPrefix(text, heading) {
			after, valid := contentAfterProjectHeading(text[len(heading):])
			if !valid || !strings.HasPrefix(strings.TrimSpace(after), "<INSTRUCTIONS>") {
				return "", 0
			}
			if end := strings.Index(text, "</INSTRUCTIONS>"); end >= 0 {
				return "project", end + len("</INSTRUCTIONS>")
			}
		}
	}
	for _, tag := range []struct{ opening, closing, category string }{
		{"<environment_context>", "</environment_context>", "environment"},
		{"<skills_instructions>", "</skills_instructions>", "skills"},
		{"<system-reminder>", "</system-reminder>", "reminder"},
		{"<permissions instructions>", "</permissions instructions>", "agent"},
		{"<collaboration_mode>", "</collaboration_mode>", "agent"},
		{"<multi_agent_role>", "</multi_agent_role>", "agent"},
		{"<multi_agent_mode>", "</multi_agent_mode>", "agent"},
	} {
		if strings.HasPrefix(text, tag.opening) {
			if end := strings.Index(text[len(tag.opening):], tag.closing); end >= 0 {
				return tag.category, len(tag.opening) + end + len(tag.closing)
			}
		}
	}
	return "", 0
}

func contentSkillsHeading(line string) bool {
	return line == "# Skills" || line == "## Skills" || line == "### Available skills"
}

func contentSectionEnd(text string, start, afterHeading int) int {
	line := strings.TrimSpace(text[start:afterHeading])
	level := len(line) - len(strings.TrimLeft(line, "#"))
	for offset := afterHeading; offset < len(text); {
		end := strings.IndexByte(text[offset:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += offset + 1
		}
		next := strings.TrimSpace(text[offset:end])
		nextLevel := len(next) - len(strings.TrimLeft(next, "#"))
		if nextLevel > 0 && nextLevel <= level && len(next) > nextLevel && next[nextLevel] == ' ' {
			return offset
		}
		offset = end
	}
	return len(text)
}

func (a *promptContentAnalyzer) readAuxiliary(value map[string]any, path, role, group string) {
	for _, key := range []string{"encrypted_content", "signature", "thoughtSignature", "thought_signature", "reasoning_signature"} {
		if raw, exists := value[key]; exists {
			a.filtered("encrypted", raw)
		}
	}
	for _, key := range []string{"metadata", "client_metadata", "cache_control", "logprobs", "token_ids", "usage", "usageMetadata", "annotations", "id", "model", "object", "status", "stop_reason", "stop_sequence", "finish_reason", "index"} {
		if raw, exists := value[key]; exists {
			a.addBlock("metadata", role, contentChild(path, key), "", raw, "", "", group)
		}
	}
}

func (a *promptContentAnalyzer) readRootMetadata(value map[string]any, path string) {
	for _, key := range sortedJSONKeys(value) {
		raw := value[key]
		if _, text := raw.(string); text && contentRootTextKey(key) {
			continue
		}
		switch key {
		case "messages", "input", "contents", "output", "output_text", "choices", "candidates", "request", "response", "requests",
			"instructions", "system", "systemInstruction", "system_instruction", "tools", "functions", "content", "text", "prompt",
			"encrypted_content", "signature", "thoughtSignature", "thought_signature", "reasoning_signature",
			"metadata", "client_metadata", "cache_control", "logprobs", "token_ids", "usage", "usageMetadata", "annotations", "id", "model", "object", "status", "stop_reason", "stop_sequence", "finish_reason", "index":
			continue
		}
		if contentMultimodalKey(key) {
			a.filtered("media", raw)
		} else {
			a.addBlock("metadata", "", contentChild(path, key), "", raw, "", "", "")
		}
	}
}

func (a *promptContentAnalyzer) cleanProtocolObject(value map[string]any) map[string]any {
	result := make(map[string]any)
	for key, raw := range value {
		switch key {
		case "encrypted_content", "signature", "thoughtSignature", "thought_signature", "reasoning_signature":
			a.filtered("encrypted", raw)
		default:
			result[key] = contentClone(raw)
		}
	}
	return result
}

func (a *promptContentAnalyzer) cleanProtocolValue(value any) any {
	if values, ok := value.([]any); ok {
		result := make([]any, 0, len(values))
		for _, raw := range values {
			if object, ok := raw.(map[string]any); ok {
				result = append(result, a.cleanProtocolObject(object))
			} else {
				result = append(result, contentClone(raw))
			}
		}
		return result
	}
	if object, ok := value.(map[string]any); ok {
		return a.cleanProtocolObject(object)
	}
	return contentClone(value)
}

func (a *promptContentAnalyzer) addBlock(category, role, path, text string, data any, callID, name, group string) {
	if text == "" && data == nil {
		return
	}
	// Identify conversation messages before filtering. Disabling user or model
	// text must not shift the truncation window to a different tool interval.
	if group != "" && contentDialogueBlock(category, text, data) {
		a.dialogueGroups[group] = true
	}
	if category == "encrypted" || category == "media" || !a.policy.Keep[category] {
		if text != "" {
			a.filtered(category, text)
		}
		if data != nil {
			a.filtered(category, data)
		}
		return
	}
	a.document.Blocks = append(a.document.Blocks, PromptContentBlock{Category: category, Role: role, Path: path, Text: text, Data: contentClone(data), CallID: callID, Name: name, Group: group})
}

func (a *promptContentAnalyzer) filtered(category string, value any) {
	if category == "" {
		category = "metadata"
	}
	size := 0
	if text, ok := value.(string); ok {
		size = len(text)
	} else if encoded, err := json.Marshal(value); err == nil {
		size = len(encoded)
	}
	count := a.document.Filtered[category]
	count.Count++
	count.Bytes += size
	a.document.Filtered[category] = count
}

func (a *promptContentAnalyzer) messageGroup(path string) string {
	group := path
	if group == "" {
		group = "/"
	}
	a.registerGroup(group)
	return group
}

func (a *promptContentAnalyzer) registerGroup(group string) {
	if group != "" && !a.groupSet[group] {
		a.groupSet[group] = true
		a.groups = append(a.groups, group)
	}
}

func (a *promptContentAnalyzer) limitMessages(maxMessages int) {
	if len(a.dialogueGroups) <= maxMessages {
		return
	}
	a.document.Truncated = true
	retained := make(map[string]bool)
	remaining := len(a.dialogueGroups)
	for _, group := range a.groups {
		if a.dialogueGroups[group] {
			remaining--
		}
		if remaining < maxMessages {
			// Tool calls/results and other auxiliary groups inside the retained
			// conversation interval stay available without consuming a slot.
			retained[group] = true
		}
	}
	blocks := make([]PromptContentBlock, 0, len(a.document.Blocks))
	for _, block := range a.document.Blocks {
		if block.Group == "" || retained[block.Group] {
			blocks = append(blocks, block)
		} else if block.Text != "" {
			a.filtered(block.Category, block.Text)
		} else if block.Data != nil {
			a.filtered(block.Category, block.Data)
		}
	}
	a.document.Blocks = blocks
}

func contentDialogueBlock(category, text string, data any) bool {
	return (category == "user" || category == "assistant") && (strings.TrimSpace(text) != "" || data != nil)
}

func contentRoleCategory(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "model":
		return "assistant"
	case "thinking":
		return "thinking"
	case "system", "developer", "instructions":
		return strings.ToLower(strings.TrimSpace(role))
	case "tool", "function":
		return "tool_result"
	default:
		return "user"
	}
}

func contentToolResultKind(kind string) bool {
	return kind == "tool_result" || strings.HasSuffix(kind, "_tool_result") || strings.HasSuffix(kind, "_call_output") || kind == "function_response" || kind == "tool_search_output"
}

func contentStateKind(kind string) bool {
	switch kind {
	case "item_reference", "computer_initialize_state", "compaction_trigger":
		return true
	default:
		return false
	}
}

func contentRootTextKey(key string) bool {
	if key == "naturalprompt" || key == "natural_prompt" {
		return true
	}
	for _, candidate := range contentMediaRootKeys {
		if key == candidate && key != "input" && key != "request" && key != "image" && key != "images" {
			return true
		}
	}
	return false
}

func contentToolCallKind(kind string) bool {
	return kind == "tool_use" || strings.HasSuffix(kind, "_tool_use") || strings.HasSuffix(kind, "_call") || kind == "function" || kind == "custom_tool_call"
}

func contentServerResultKind(kind string) bool {
	switch kind {
	case "web_search_result", "web_fetch_result", "web_search_tool_result_error", "web_fetch_tool_result_error", "tool_reference", "search_result":
		return true
	default:
		return false
	}
}

func contentRetainedDocument(value map[string]any) bool {
	if contentInt(value["schema_version"]) != promptContentSchemaVersion {
		return false
	}
	blocks, ok := value["blocks"].([]any)
	if !ok {
		return false
	}
	if _, ok := value["filtered"].(map[string]any); !ok {
		return false
	}
	for key := range value {
		if key != "schema_version" && key != "blocks" && key != "filtered" && key != "truncated" {
			return false
		}
	}
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok || !contentCategory(contentString(block, "category")) {
			return false
		}
		for _, key := range []string{"role", "path", "text", "call_id", "name", "group"} {
			if raw, exists := block[key]; exists {
				if _, ok := raw.(string); !ok {
					return false
				}
			}
		}
	}
	return true
}

func contentCategory(category string) bool {
	switch category {
	case "user", "assistant", "tool_call", "tool_result", "system", "developer", "instructions", "project", "agent", "environment", "skills", "reminder", "thinking", "reasoning_summary", "tool_definition", "metadata", "encrypted", "media":
		return true
	default:
		return false
	}
}

func contentMessageObject(value map[string]any) bool {
	return value["role"] != nil || value["type"] != nil && value["messages"] == nil && value["input"] == nil && value["output"] == nil && value["response"] == nil && value["request"] == nil
}

func contentAuxiliaryItem(value any) bool {
	item, ok := value.(map[string]any)
	if !ok {
		return false
	}
	kind := strings.ToLower(contentString(item, "type"))
	return kind == "reasoning" || kind == "thinking" || kind == "compaction_summary" || kind == "additional_tools" || contentStateKind(kind) || kind == "redacted_thinking" || contentMediaKind(kind) || contentMediaPart(item)
}

func contentTextKind(kind string) bool {
	return kind == "text" || kind == "input_text" || kind == "output_text" || kind == "summary_text"
}

func contentMediaKind(kind string) bool {
	switch kind {
	case "image", "image_url", "input_image", "output_image", "audio", "audio_url", "input_audio", "output_audio",
		"video", "video_url", "input_video", "output_video", "file", "input_file", "output_file", "document":
		return true
	default:
		return false
	}
}

func contentMultimodalKey(key string) bool {
	normalized := strings.NewReplacer("-", "_", " ", "_").Replace(strings.ToLower(strings.TrimSpace(key)))
	switch normalized {
	case "image", "images", "image_url", "image_urls", "image_data", "input_image", "input_images", "output_image", "output_images",
		"reference_image", "reference_images", "source_image", "source_images", "mask", "mask_image", "mask_image_url", "input_image_mask",
		"audio", "audios", "audio_url", "audio_urls", "audio_data", "input_audio", "input_audios", "output_audio", "output_audios",
		"video", "videos", "video_url", "video_urls", "video_data", "input_video", "input_videos", "output_video", "output_videos",
		"file", "files", "file_url", "file_urls", "file_data", "input_file", "input_files", "output_file", "output_files",
		"document", "documents", "inline_data", "inlinedata", "filedata", "blob", "binary", "bytes", "attachments":
		return true
	default:
		return false
	}
}

// A project heading is valid only with its actual heading grammar followed by
// a newline and an instruction block. Quoted prose stays readable user content.
func contentAfterProjectHeading(suffix string) (string, bool) {
	heading, rest, found := strings.Cut(suffix, "\n")
	if !found {
		return "", false
	}
	heading = strings.TrimRight(heading, "\r")
	if heading != "" && (!strings.HasPrefix(heading, " for ") || strings.TrimSpace(strings.TrimPrefix(heading, " for ")) == "") {
		return "", false
	}
	return rest, true
}

func contentNonMessageItem(value any) bool {
	item, ok := value.(map[string]any)
	if !ok {
		return false
	}
	kind := strings.ToLower(contentString(item, "type"))
	return kind == "additional_tools" || contentStateKind(kind)
}

func contentNestedToolPayload(value map[string]any) bool {
	for _, key := range []string{"function", "custom", "functionCall", "function_call", "functionResponse", "function_response"} {
		if _, ok := value[key].(map[string]any); ok {
			return true
		}
	}
	return false
}

func contentMediaPart(value map[string]any) bool {
	for _, key := range []string{"inlineData", "inline_data", "fileData", "file_data", "image_url", "input_audio", "videoMetadata"} {
		if value[key] != nil {
			return true
		}
	}
	for _, key := range []string{"mimeType", "mime_type", "mime", "media_type"} {
		mime := strings.ToLower(contentString(value, key))
		if strings.HasPrefix(mime, "image/") || strings.HasPrefix(mime, "audio/") || strings.HasPrefix(mime, "video/") || strings.HasPrefix(mime, "application/") {
			return true
		}
	}
	return false
}

func contentAgentIdentities(document any) map[string]bool {
	identities := make(map[string]bool)
	var inspectText func(any)
	inspectText = func(value any) {
		switch typed := value.(type) {
		case string:
			fence := ""
			for _, line := range strings.Split(typed, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
					marker := line[:3]
					if fence == "" {
						fence = marker
					} else if fence == marker {
						fence = ""
					}
					continue
				}
				if fence != "" {
					continue
				}
				if strings.HasPrefix(line, "You are Codex") {
					identities["codex"] = true
				}
				if strings.HasPrefix(line, "You are Claude Code") {
					identities["claude"] = true
				}
			}
		case []any:
			for _, raw := range typed {
				inspectText(raw)
			}
		case map[string]any:
			for _, key := range []string{"text", "content", "parts"} {
				if raw, exists := typed[key]; exists {
					inspectText(raw)
				}
			}
		}
	}
	var inspect func(any)
	inspect = func(value any) {
		switch typed := value.(type) {
		case []any:
			for _, raw := range typed {
				inspect(raw)
			}
		case map[string]any:
			role := contentString(typed, "role")
			if role == "system" || role == "developer" {
				inspectText(typed["content"])
			}
			for _, key := range []string{"instructions", "system", "systemInstruction", "system_instruction"} {
				inspectText(typed[key])
			}
			for _, key := range []string{"messages", "input", "contents", "request", "response", "requests"} {
				inspect(typed[key])
			}
		}
	}
	inspect(document)
	return identities
}

func contentChild(path, key string) string       { return path + "/" + key }
func contentIndex(path string, index int) string { return fmt.Sprintf("%s/%d", path, index) }
func contentString(value map[string]any, key string) string {
	text, _ := value[key].(string)
	return text
}
func contentFirstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := contentString(value, key); text != "" {
			return text
		}
	}
	return ""
}
func contentSlice(value any) []any {
	if values, ok := value.([]any); ok {
		return values
	}
	if value != nil {
		return []any{value}
	}
	return nil
}
func contentInt(value any) int {
	switch number := value.(type) {
	case int:
		return number
	case float64:
		return int(number)
	case json.Number:
		integer, _ := number.Int64()
		return int(integer)
	default:
		return 0
	}
}
func contentClone(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clone := make(map[string]any, len(typed))
		for key, raw := range typed {
			clone[key] = contentClone(raw)
		}
		return clone
	case []any:
		clone := make([]any, len(typed))
		for index, raw := range typed {
			clone[index] = contentClone(raw)
		}
		return clone
	default:
		return value
	}
}
