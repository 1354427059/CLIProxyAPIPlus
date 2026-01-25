package claude

import (
	"bytes"
	"testing"
)

func TestClaudeToOrchidsRequest(t *testing.T) {
	in := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	out := ConvertClaudeRequestToOrchids("claude-sonnet-4-5", in, true)
	if !bytes.Contains(out, []byte(`"type":"user_request"`)) {
		t.Fatalf("missing user_request")
	}
}
