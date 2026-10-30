package catalog

import "testing"

func TestModelsListed(t *testing.T) {
	models := Models()
	if len(models) < 8 {
		t.Fatalf("expected the aliases and versions, got %d", len(models))
	}
	seen := map[string]bool{}
	for _, m := range models {
		if m.ID == "" || m.Label == "" {
			t.Errorf("incomplete entry: %+v", m)
		}
		if seen[m.ID] {
			t.Errorf("duplicate model %q", m.ID)
		}
		seen[m.ID] = true
	}
	for _, want := range []string{"opus", "sonnet", "claude-opus-5-5", "claude-sonnet-5-5"} {
		if !seen[want] {
			t.Errorf("%q is missing from the catalog", want)
		}
	}
}

func TestLabel(t *testing.T) {
	if got := Label("claude-sonnet-5-5"); got != "Sonnet 5.5" {
		t.Errorf("got %q", got)
	}
	if got := Label("some-gateway-model"); got != "some-gateway-model" {
		t.Errorf("an unknown ID reads back unchanged, got %q", got)
	}
}

func TestIsModelField(t *testing.T) {
	for _, path := range []string{"model", "advisorModel", "availableModels", "deniedModels", "agent.model"} {
		if !IsModelField(path) {
			t.Errorf("%q names models", path)
		}
	}
	for _, path := range []string{"permissions.allow", "sandbox.enabled", "theme"} {
		if IsModelField(path) {
			t.Errorf("%q does not name models", path)
		}
	}
}
