package registry

import "testing"

func TestGetOrchidsModelsIncludesDefault(t *testing.T) {
	models := GetOrchidsModels()
	if len(models) == 0 {
		t.Fatalf("expected orchids models")
	}
	found := false
	for _, model := range models {
		if model != nil && model.ID == "claude-sonnet-4-5" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing default orchids model")
	}
}
