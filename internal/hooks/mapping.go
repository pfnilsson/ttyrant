package hooks

import "github.com/pfnilsson/ttyrant/internal/model"

// MapEventToStatus maps a Claude Code hook event name to a SessionStatus.
func MapEventToStatus(event string) model.SessionStatus {
	switch event {
	case "SessionStart":
		return model.StatusStarting
	case "UserPromptSubmit":
		return model.StatusWorking
	case "PreToolUse":
		return model.StatusWorking
	case "PostToolUse":
		return model.StatusWorking
	case "PostToolUseFailure":
		return model.StatusWorking
	case "SubagentStart":
		return model.StatusWorking
	case "SubagentStop":
		return model.StatusWorking
	case "ElicitationResult":
		return model.StatusWorking
	case "PermissionRequest":
		return model.StatusNeedsInput
	case "Elicitation":
		return model.StatusNeedsInput
	case "TaskCompleted":
		return model.StatusDone
	case "Stop":
		return model.StatusDone
	case "SessionEnd":
		return model.StatusExited
	default:
		return model.StatusUnknown
	}
}

// WaitingReason returns the waiting reason for events that produce needs_input.
func WaitingReason(event string) string {
	switch event {
	case "PermissionRequest":
		return "permission"
	case "Elicitation":
		return "elicitation"
	}
	return ""
}
