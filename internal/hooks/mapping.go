package hooks

import "github.com/pfnilsson/ttyrant/internal/model"

type transitionKey struct {
	from  model.SessionStatus
	event string
}

// transitions defines allowed state transitions. If a (from, event) pair
// is absent, the event is rejected and the current state is preserved.
var transitions = map[transitionKey]model.SessionStatus{
	// From Starting
	{model.StatusStarting, "UserPromptSubmit"}:   model.StatusWorking,
	{model.StatusStarting, "PreToolUse"}:         model.StatusWorking,
	{model.StatusStarting, "PostToolUse"}:        model.StatusWorking,
	{model.StatusStarting, "PostToolUseFailure"}: model.StatusWorking,
	{model.StatusStarting, "SubagentStart"}:      model.StatusWorking,
	{model.StatusStarting, "SubagentStop"}:       model.StatusWorking,
	{model.StatusStarting, "ElicitationResult"}:  model.StatusWorking,
	{model.StatusStarting, "TaskCompleted"}:      model.StatusWorking,
	{model.StatusStarting, "PermissionRequest"}:  model.StatusNeedsInput,
	{model.StatusStarting, "Elicitation"}:        model.StatusNeedsInput,
	{model.StatusStarting, "Stop"}:               model.StatusDone,
	{model.StatusStarting, "SessionEnd"}:         model.StatusExited,

	// From Working — everything is reachable
	{model.StatusWorking, "UserPromptSubmit"}:   model.StatusWorking,
	{model.StatusWorking, "PreToolUse"}:         model.StatusWorking,
	{model.StatusWorking, "PostToolUse"}:        model.StatusWorking,
	{model.StatusWorking, "PostToolUseFailure"}: model.StatusWorking,
	{model.StatusWorking, "SubagentStart"}:      model.StatusWorking,
	{model.StatusWorking, "SubagentStop"}:       model.StatusWorking,
	{model.StatusWorking, "ElicitationResult"}:  model.StatusWorking,
	{model.StatusWorking, "TaskCompleted"}:      model.StatusWorking,
	{model.StatusWorking, "PermissionRequest"}:  model.StatusNeedsInput,
	{model.StatusWorking, "Elicitation"}:        model.StatusNeedsInput,
	{model.StatusWorking, "Stop"}:               model.StatusDone,
	{model.StatusWorking, "SessionEnd"}:         model.StatusExited,

	// From NeedsInput — only new-work events and session lifecycle clear it.
	// PostToolUse, SubagentStop, TaskCompleted, PostToolUseFailure are
	// rejected to prevent late-arriving completion events from overriding.
	{model.StatusNeedsInput, "UserPromptSubmit"}:  model.StatusWorking,
	{model.StatusNeedsInput, "PreToolUse"}:        model.StatusWorking,
	{model.StatusNeedsInput, "SubagentStart"}:     model.StatusWorking,
	{model.StatusNeedsInput, "ElicitationResult"}: model.StatusWorking,
	{model.StatusNeedsInput, "PermissionRequest"}: model.StatusNeedsInput,
	{model.StatusNeedsInput, "Elicitation"}:       model.StatusNeedsInput,
	{model.StatusNeedsInput, "SessionStart"}:      model.StatusStarting,
	{model.StatusNeedsInput, "SessionEnd"}:        model.StatusExited,

	// From Done — only explicit new-work or session events
	{model.StatusDone, "UserPromptSubmit"}: model.StatusWorking,
	{model.StatusDone, "PreToolUse"}:       model.StatusWorking,
	{model.StatusDone, "SessionStart"}:     model.StatusStarting,
	{model.StatusDone, "SessionEnd"}:       model.StatusExited,

	// From Exited
	{model.StatusExited, "UserPromptSubmit"}: model.StatusWorking,
	{model.StatusExited, "PreToolUse"}:       model.StatusWorking,
	{model.StatusExited, "SessionStart"}:     model.StatusStarting,
	{model.StatusExited, "SessionEnd"}:       model.StatusExited,
}

// Transition returns the next status for the given (current, event) pair.
// If the transition is not allowed, it returns the current status and false.
func Transition(current model.SessionStatus, event string) (model.SessionStatus, bool) {
	next, ok := transitions[transitionKey{current, event}]
	if !ok {
		return current, false
	}
	return next, true
}

// MapEventToStatus maps a hook event to its initial status, used when
// there is no existing state for this cwd.
func MapEventToStatus(event string) model.SessionStatus {
	switch event {
	case "SessionStart":
		return model.StatusStarting
	case "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure",
		"SubagentStart", "SubagentStop", "ElicitationResult", "TaskCompleted":
		return model.StatusWorking
	case "PermissionRequest", "Elicitation":
		return model.StatusNeedsInput
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
