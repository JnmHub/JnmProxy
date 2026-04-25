package singbox

import (
	"encoding/json"
	"strings"
)

var sensitiveKeys = map[string]struct{}{
	"password":    {},
	"uuid":        {},
	"id":          {},
	"token":       {},
	"private_key": {},
	"server_key":  {},
	"short_id":    {},
	"public_key":  {},
	"pbk":         {},
	"auth":        {},
}

func RedactJSON(rawJSON string) string {
	var value any
	if err := json.Unmarshal([]byte(rawJSON), &value); err != nil {
		return "<invalid-json>"
	}
	redacted := redactValue(value)
	content, err := json.Marshal(redacted)
	if err != nil {
		return "<invalid-json>"
	}
	return string(content)
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if _, ok := sensitiveKeys[strings.ToLower(key)]; ok {
				result[key] = "***"
				continue
			}
			result[key] = redactValue(item)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, redactValue(item))
		}
		return result
	default:
		return value
	}
}
