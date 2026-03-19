package doctor

import (
	"testing"
)

func TestRunAll_ReturnsResults(t *testing.T) {
	results := RunAll()
	if len(results) == 0 {
		t.Fatal("expected at least one check result")
	}

	// Verify we have all expected checks.
	names := make(map[string]bool)
	for _, r := range results {
		names[r.Name] = true
		// Every result should have a non-empty message.
		if r.Message == "" {
			t.Errorf("check %q has empty message", r.Name)
		}
	}

	expected := []string{
		"ttyrant in PATH",
		"State directory",
		"State files readable",
		"Hooks in Claude Code settings",
		"Process scanner",
		"Claude Code processes",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing check: %s", name)
		}
	}
}

func TestPrint_AllPass(t *testing.T) {
	results := []CheckResult{
		{"Test check", true, "all good"},
	}
	allOK := Print(results)
	if !allOK {
		t.Error("expected allOK = true")
	}
}

func TestPrint_SomeFail(t *testing.T) {
	results := []CheckResult{
		{"Good check", true, "ok"},
		{"Bad check", false, "not ok"},
	}
	allOK := Print(results)
	if allOK {
		t.Error("expected allOK = false")
	}
}
