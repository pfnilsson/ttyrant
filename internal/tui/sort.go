package tui

import (
	"sort"

	"github.com/pfnilsson/ttyrant/internal/model"
)

// statusPriority returns a sort rank where lower = more important.
func statusPriority(s model.SessionStatus) int {
	switch s {
	case model.StatusNeedsInput:
		return 0
	case model.StatusWorking:
		return 1
	case model.StatusStarting:
		return 2
	case model.StatusReady:
		return 3
	case model.StatusDone:
		return 4
	case model.StatusUnknown:
		return 5
	case model.StatusExited:
		return 6
	case model.StatusActive:
		return 7
	default:
		return 8
	}
}

func sortRows(rows []model.SessionRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		pi, pj := statusPriority(rows[i].Status), statusPriority(rows[j].Status)
		if pi != pj {
			return pi < pj
		}
		return rows[i].LastEventAt.After(rows[j].LastEventAt)
	})
}
