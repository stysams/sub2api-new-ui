package securityaudit

import (
	"bytes"
	"unicode/utf8"
)

const PromptResponseTextLimit = 256 * 1024

// completeResponseJSONPrefix recovers a bounded JSON capture cut inside a text
// value. Invalid prefixes remain invalid; callers store an incomplete status.
func completeResponseJSONPrefix(body []byte) []byte {
	closers := make([]byte, 0, 16)
	inString, escaped, stringStart := false, false, 0
	for i, b := range body {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch b {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString, stringStart = true, i
		case '{':
			closers = append(closers, '}')
		case '[':
			closers = append(closers, ']')
		case '}', ']':
			if len(closers) == 0 || closers[len(closers)-1] != b {
				return nil
			}
			closers = closers[:len(closers)-1]
		}
		if len(closers) > 256 {
			return nil
		}
	}
	prefix := append([]byte(nil), body...)
	if inString {
		// Remove an unfinished escape (including a partial Unicode escape).
		for i := stringStart + 1; i < len(prefix); i++ {
			if prefix[i] != '\\' {
				continue
			}
			if i+1 >= len(prefix) || (prefix[i+1] == 'u' && i+6 > len(prefix)) {
				prefix = prefix[:i]
				break
			}
			if prefix[i+1] == 'u' {
				i += 5
			} else {
				i++
			}
		}
		for len(prefix) > stringStart+1 && !utf8.Valid(prefix[stringStart+1:]) {
			prefix = prefix[:len(prefix)-1]
		}
		prefix = append(prefix, '"')
	}
	prefix = bytes.TrimSpace(prefix)
	if len(prefix) > 0 && prefix[len(prefix)-1] == ',' {
		prefix = bytes.TrimSpace(prefix[:len(prefix)-1])
	}
	if len(prefix) > 0 && prefix[len(prefix)-1] == ':' {
		prefix = append(prefix, "null"...)
	}
	for i := len(closers) - 1; i >= 0; i-- {
		prefix = append(prefix, closers[i])
	}
	return prefix
}

func truncateUTF8(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value, true
}
