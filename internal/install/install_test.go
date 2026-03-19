package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	config := generateConfig("/usr/local/bin/ttyrant hook")

	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		t.Fatal("expected hooks map")
	}

	// All expected events should be present.
	for _, event := range hookEvents {
		groups, ok := hooks[event]
		if !ok {
			t.Errorf("missing event: %s", event)
			continue
		}
		arr, ok := groups.([]matcherGroup)
		if !ok || len(arr) == 0 {
			t.Errorf("event %s: expected non-empty matcher group array", event)
			continue
		}
		if arr[0].Hooks[0].Command != "/usr/local/bin/ttyrant hook" {
			t.Errorf("event %s: command = %q", event, arr[0].Hooks[0].Command)
		}
	}
}

func TestInstall_PrintOnly(t *testing.T) {
	// Just verify it doesn't error.
	// Capture stdout by redirecting - not worth the complexity, just check no panic.
	err := Install(true)
	if err != nil {
		t.Fatalf("Install(printOnly=true): %v", err)
	}
}

func TestInstall_WritesSettings(t *testing.T) {
	tmp := t.TempDir()
	settingsDir := filepath.Join(tmp, ".claude")
	settingsFile := filepath.Join(settingsDir, "settings.json")

	// Override settingsPath for testing.
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	err := Install(false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("expected hooks in settings")
	}

	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("missing SessionStart hook")
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("missing PreToolUse hook")
	}
}

func TestInstall_MergesWithExisting(t *testing.T) {
	tmp := t.TempDir()
	settingsDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(settingsDir, 0o755)
	settingsFile := filepath.Join(settingsDir, "settings.json")

	// Write existing settings with a custom hook.
	existing := map[string]any{
		"other_setting": true,
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "my-custom-hook.sh",
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(settingsFile, data, 0o644)

	t.Setenv("HOME", tmp)

	err := Install(false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, _ = os.ReadFile(settingsFile)
	var settings map[string]any
	json.Unmarshal(data, &settings)

	// other_setting should be preserved.
	if settings["other_setting"] != true {
		t.Error("existing setting was lost")
	}

	hooks := settings["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)

	// Should have the custom hook + ttyrant hook.
	if len(preToolUse) < 2 {
		t.Errorf("expected at least 2 PreToolUse groups, got %d", len(preToolUse))
	}
}

func TestUninstall(t *testing.T) {
	tmp := t.TempDir()
	settingsDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(settingsDir, 0o755)
	settingsFile := filepath.Join(settingsDir, "settings.json")

	t.Setenv("HOME", tmp)

	// Install first.
	if err := Install(false); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Verify hooks are there.
	data, _ := os.ReadFile(settingsFile)
	var before map[string]any
	json.Unmarshal(data, &before)
	if _, ok := before["hooks"]; !ok {
		t.Fatal("hooks not installed")
	}

	// Uninstall.
	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	data, _ = os.ReadFile(settingsFile)
	var after map[string]any
	json.Unmarshal(data, &after)

	// hooks key should be gone (all were ttyrant).
	if _, ok := after["hooks"]; ok {
		t.Error("hooks should have been removed")
	}
}

func TestUninstall_PreservesOtherHooks(t *testing.T) {
	tmp := t.TempDir()
	settingsDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(settingsDir, 0o755)
	settingsFile := filepath.Join(settingsDir, "settings.json")

	// Settings with both custom and ttyrant hooks.
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "my-hook.sh"},
					},
				},
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ttyrant hook"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsFile, data, 0o644)

	t.Setenv("HOME", tmp)

	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}

	data, _ = os.ReadFile(settingsFile)
	var after map[string]any
	json.Unmarshal(data, &after)

	hooks := after["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Errorf("expected 1 remaining group, got %d", len(preToolUse))
	}
}

func TestIsTtyrantCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"ttyrant hook", true},
		{"/usr/local/bin/ttyrant hook", true},
		{"/home/user/go/bin/ttyrant hook", true},
		// Legacy format should still be recognized for clean uninstall.
		{"ttyrant-hook", true},
		{"/usr/local/bin/ttyrant-hook", true},
		{"my-custom-hook.sh", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isTtyrantCommand(tt.cmd); got != tt.want {
			t.Errorf("isTtyrantCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

func TestUninstall_NoSettings(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Should not error when no settings file exists.
	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall with no settings: %v", err)
	}
}
