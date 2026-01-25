package orchids

import (
	"encoding/base64"
	"testing"
)

func TestParseOrchidsClientJWT(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"rotating_token":"rot"}`))
	jwt := header + "." + payload + ".sig"

	cred, err := ParseCredentials(jwt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred.ClientJWT != jwt {
		t.Fatalf("expected client jwt to match input")
	}
	if cred.RotatingToken != "rot" {
		t.Fatalf("expected rotating token to be extracted")
	}
}
