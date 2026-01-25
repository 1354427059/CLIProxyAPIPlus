package executor

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestOrchidsExecutorIdentifier(t *testing.T) {
	exec := NewOrchidsExecutor(&config.Config{})
	if exec.Identifier() != "orchids" {
		t.Fatalf("identifier mismatch")
	}
}
