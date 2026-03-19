package install

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// hookEvents lists all Claude Code hook events ttyrant monitors.
var hookEvents = []string{
	"SessionStart",
	"SessionEnd",
	"UserPromptSubmit",
	"PreToolUse",
	"PostToolUse",
	"PostToolUseFailure",
	"PermissionRequest",
	"Elicitation",
	"ElicitationResult",
	"SubagentStart",
	"SubagentStop",
	"TaskCompleted",
	"Stop",
}

// settingsPath returns the path to the Claude Code user settings file.
func settingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// hookCommand returns the ttyrant hook command string.
// Uses the full path if found, otherwise assumes it's in PATH.
func hookCommand() string {
	path, err := exec.LookPath("ttyrant")
	if err == nil {
		abs, err := filepath.Abs(path)
		if err == nil {
			return abs + " hook"
		}
		return path + " hook"
	}
	return "ttyrant hook"
}

// hookEntry is a single hook definition within a matcher group.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// matcherGroup represents a matcher + its hooks array.
type matcherGroup struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []hookEntry `json:"hooks"`
}

// ttyrantMarker is used to identify our hooks in settings.json.
const ttyrantMarker = "ttyrant hook"

// Install adds ttyrant hooks to Claude Code settings.json.
// If print is true, it prints the config instead of writing it.
func Install(printOnly bool) error {
	cmd := hookCommand()
	config := generateConfig(cmd)

	if printOnly {
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Read existing settings.
	existing, err := readSettings()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read settings: %w", err)
	}
	if existing == nil {
		existing = make(map[string]any)
	}

	// Merge hooks into existing settings.
	mergeHooks(existing, config)

	return writeSettings(existing)
}

// Uninstall removes ttyrant hooks from Claude Code settings.json.
func Uninstall() error {
	existing, err := readSettings()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to uninstall
		}
		return fmt.Errorf("read settings: %w", err)
	}

	removeHooks(existing)
	return writeSettings(existing)
}

// IsInstalled checks whether ttyrant hooks are present in settings.
func IsInstalled() bool {
	settings, err := readSettings()
	if err != nil {
		return false
	}
	hooks, ok := settings["hooks"]
	if !ok {
		return false
	}

	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		return false
	}

	for _, event := range hookEvents {
		groups, ok := hooksMap[event]
		if !ok {
			return false
		}
		if !containsTtyrantHook(groups) {
			return false
		}
	}
	return true
}

func generateConfig(cmd string) map[string]any {
	hooks := make(map[string]any)
	for _, event := range hookEvents {
		hooks[event] = []matcherGroup{
			{
				Hooks: []hookEntry{
					{Type: "command", Command: cmd},
				},
			},
		}
	}
	return map[string]any{"hooks": hooks}
}

func mergeHooks(existing, config map[string]any) {
	configHooks := config["hooks"].(map[string]any)

	existingHooks, ok := existing["hooks"].(map[string]any)
	if !ok {
		existing["hooks"] = configHooks
		return
	}

	for event, newGroups := range configHooks {
		existingGroups, ok := existingHooks[event]
		if !ok {
			existingHooks[event] = newGroups
			continue
		}

		// Remove any existing ttyrant entries first, then append ours.
		cleaned := removeTtyrantFromGroups(existingGroups)
		newGroupsList := newGroups.([]matcherGroup)

		// Convert cleaned back to []any and append.
		var merged []any
		if arr, ok := cleaned.([]any); ok {
			merged = arr
		}
		for _, g := range newGroupsList {
			merged = append(merged, g)
		}
		existingHooks[event] = merged
	}
}

func removeHooks(settings map[string]any) {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return
	}

	for event, groups := range hooks {
		cleaned := removeTtyrantFromGroups(groups)
		if arr, ok := cleaned.([]any); ok && len(arr) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = cleaned
		}
	}

	// If hooks map is empty, remove it entirely.
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
}

func removeTtyrantFromGroups(groups any) any {
	arr, ok := groups.([]any)
	if !ok {
		return groups
	}

	var result []any
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			result = append(result, item)
			continue
		}

		innerHooks, ok := m["hooks"].([]any)
		if !ok {
			result = append(result, item)
			continue
		}

		var filtered []any
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if !ok {
				filtered = append(filtered, h)
				continue
			}
			cmd, _ := hm["command"].(string)
			if !isTtyrantCommand(cmd) {
				filtered = append(filtered, h)
			}
		}

		if len(filtered) > 0 {
			m["hooks"] = filtered
			result = append(result, m)
		}
		// If all hooks in the group were ttyrant, drop the whole group.
	}

	return result
}

func containsTtyrantHook(groups any) bool {
	arr, ok := groups.([]any)
	if !ok {
		return false
	}

	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		innerHooks, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hm["command"].(string)
			if isTtyrantCommand(cmd) {
				return true
			}
		}
	}
	return false
}

func isTtyrantCommand(cmd string) bool {
	// Match "ttyrant hook", "/path/to/ttyrant hook", and legacy "ttyrant-hook" / "/path/to/ttyrant-hook".
	if len(cmd) >= len(ttyrantMarker) && cmd[len(cmd)-len(ttyrantMarker):] == ttyrantMarker {
		return true
	}
	const legacyMarker = "ttyrant-hook"
	if len(cmd) >= len(legacyMarker) && cmd[len(cmd)-len(legacyMarker):] == legacyMarker {
		return true
	}
	return false
}

func readSettings() (map[string]any, error) {
	path := settingsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	return settings, nil
}

func writeSettings(settings map[string]any) error {
	path := settingsPath()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}
