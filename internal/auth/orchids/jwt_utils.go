package orchids

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// ParseJWTExpiry extracts exp from JWT payload without signature validation.
func ParseJWTExpiry(jwt string) (time.Time, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return time.Time{}, false
	}
	if exp, ok := payload["exp"].(float64); ok && exp > 0 {
		return time.Unix(int64(exp), 0), true
	}
	return time.Time{}, false
}
