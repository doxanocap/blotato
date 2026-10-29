package dto

import "github.com/doxanocap/blotato/internal/domain/entity/types"

// RunStatus projects a run and one associated action for diagnostics.
type RunStatus struct {
	RunID       types.ID
	State       types.RunState
	Error       string
	Step        types.ActionStep
	Status      types.ActionStatus
	ActionError string
}
