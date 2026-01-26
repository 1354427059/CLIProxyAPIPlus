package orchids

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestParseJWTExpiry(t *testing.T) {
	jwt := buildTestJWT(map[string]any{"exp": float64(1893456000)})
	expiry, ok := ParseJWTExpiry(jwt)
	if !ok {
		t.Fatalf("expected ok for valid jwt")
	}
	if expiry.Unix() != 1893456000 {
		t.Fatalf("unexpected expiry: %v", expiry)
	}
}

func buildTestJWT(payload map[string]any) string {
	header := map[string]any{"alg": "none", "typ": "JWT"}
	return base64url(header) + "." + base64url(payload) + ".sig"
}

func base64url(v map[string]any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}
