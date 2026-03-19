package tui

import "github.com/charmbracelet/bubbletea"

type keyAction int

const (
	keyNone keyAction = iota
	keyQuit
	keyUp
	keyDown
	keyAttachTmux
	keyAttachTmux2
	keyKill
	keyWorktree
	keyOpen
)

func matchKey(msg tea.KeyMsg) keyAction {
	switch msg.Type {
	case tea.KeyCtrlC:
		return keyQuit
	default:
		switch msg.String() {
		case "q":
			return keyQuit
		case "j":
			return keyDown
		case "k":
			return keyUp
		case "a":
			return keyAttachTmux
		case "A":
			return keyAttachTmux2
		case "d":
			return keyKill
		case "w":
			return keyWorktree
		case "o":
			return keyOpen
		}
	}
	return keyNone
}
