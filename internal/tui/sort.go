package tui

import (
	"sort"

	"github.com/pfnilsson/ttyrant/internal/model"
)

func sortRows(rows []model.SessionRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].SessionName < rows[j].SessionName
	})
}
