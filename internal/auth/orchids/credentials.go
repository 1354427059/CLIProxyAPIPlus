package orchids

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// Credentials captures the Orchids client JWT and optional rotating token.
type Credentials struct {
	ClientJWT     string
	RotatingToken string
}

// ParseCredentials parses an Orchids credential string.
// Supported formats:
//   1) pure JWT
//   2) "__client=JWT" cookie string (optionally with other cookies)
//   3) "JWT|rotating_token"
func ParseCredentials(input string) (*Credentials, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, errors.New("orchids credentials: empty input")
	}

	// Extract __client cookie value when present.
	if strings.Contains(trimmed, "__client=") {
		trimmed = extractClientCookie(trimmed)
		if trimmed == "" {
			return nil, errors.New("orchids credentials: __client cookie not found")
		}
	}

	clientJWT := trimmed
	rotatingToken := ""
	if strings.Contains(trimmed, "|") {
		parts := strings.SplitN(trimmed, "|", 2)
		clientJWT = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			rotatingToken = strings.TrimSpace(parts[1])
		}
	}

	if !looksLikeJWT(clientJWT) {
		return nil, errors.New("orchids credentials: invalid jwt format")
	}

	if rotatingToken == "" {
		rotatingToken = extractRotatingToken(clientJWT)
	}

	return &Credentials{
		ClientJWT:     clientJWT,
		RotatingToken: rotatingToken,
	}, nil
}

func extractClientCookie(raw string) string {
	parts := strings.Split(raw, "__client=")
	if len(parts) < 2 {
		return ""
	}
	value := parts[1]
	if idx := strings.Index(value, ";"); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func looksLikeJWT(v string) bool {
	chunks := strings.Split(v, ".")
	return len(chunks) == 3 && chunks[0] != "" && chunks[1] != "" && chunks[2] != ""
}

func extractRotatingToken(jwt string) string {
	chunks := strings.Split(jwt, ".")
	if len(chunks) != 3 {
		return ""
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(chunks[1])
	if err != nil {
		return ""
	}
	var payload map[string]any
	if err = json.Unmarshal(payloadBytes, &payload); err != nil {
		return ""
	}
	if token, ok := payload["rotating_token"].(string); ok {
		return token
	}
	return ""
}
