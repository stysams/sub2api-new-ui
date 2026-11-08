package securityaudit

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"
)

var promptRecordingEndpoints = map[string]struct{}{
	"/v1/responses":        {},
	"/v1/messages":         {},
	"/v1/chat/completions": {},
}

// These APIs use protocol-specific request bodies and are recorded only when
// the caller and canonical endpoint agree. This keeps model-listing, cached
// content, and unrelated routes out of prompt history.
var promptRecordingProtocolEndpoints = map[string]map[string]struct{}{
	"gemini": {
		"/v1beta/models": {},
	},
	"openai_images": {
		"/v1/images/generations": {},
		"/v1/images/edits":       {},
		"/v1/videos/generations": {},
		"/v1/videos/edits":       {},
		"/v1/videos/extensions":  {},
	},
}

func promptRecordSessionID(headers http.Header) string {
	for _, name := range []string{
		"Session-Id", "X-Session-Id", "X-Claude-Code-Session-Id",
		"Conversation-Id", "X-Conversation-Id", "X-Thread-Id",
	} {
		if value := truncatePromptRecordIdentifier(strings.TrimSpace(headers.Get(name)), 128); value != "" {
			return value
		}
	}
	return ""
}

func truncatePromptRecordIdentifier(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	for maxBytes > 0 && !utf8.RuneStart(value[maxBytes]) {
		maxBytes--
	}
	return value[:maxBytes]
}

type codexTurnMetadata struct {
	RequestKind  string `json:"request_kind"`
	ThreadSource string `json:"thread_source"`
	TurnTrigger  string `json:"turn_trigger"`
}

// ShouldRecordPromptRequest limits prompt history to user-facing text APIs and
// excludes Codex background traffic identified in production request metadata.
func ShouldRecordPromptRequest(req Request) bool {
	if req.recordingExcluded {
		return false
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	if _, allowed := promptRecordingEndpoints[endpoint]; allowed {
		// Preserve the original compatibility surface for chat protocols.
		return !promptRecordingExcludedTurn(req.Headers)
	}
	protocol := strings.ToLower(strings.TrimSpace(req.Protocol))
	if _, allowed := promptRecordingProtocolEndpoints[protocol][endpoint]; !allowed {
		return false
	}
	return !promptRecordingExcludedTurn(req.Headers)
}

func promptRecordingExcludedTurn(headers http.Header) bool {
	for _, raw := range headers.Values("X-Codex-Turn-Metadata") {
		var metadata codexTurnMetadata
		if json.Unmarshal([]byte(raw), &metadata) != nil {
			continue
		}
		requestKind := strings.ToLower(strings.TrimSpace(metadata.RequestKind))
		threadSource := strings.ToLower(strings.TrimSpace(metadata.ThreadSource))
		turnTrigger := strings.ToLower(strings.TrimSpace(metadata.TurnTrigger))
		if requestKind == "memory" || requestKind == "compaction" {
			return true
		}
		if threadSource == "memory_consolidation" || threadSource == "thread_title" || threadSource == "system" || turnTrigger == "thread_title" {
			return true
		}
	}
	return false
}
