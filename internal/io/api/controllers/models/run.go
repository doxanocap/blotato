package models

import "github.com/doxanocap/blotato/internal/domain/entity/dto"

// Run projects an execution and one action without exposing a domain DTO.
type Run struct {
	RunID       int64  `json:"run_id"`
	State       string `json:"state"`
	Error       string `json:"error"`
	Step        string `json:"step"`
	Status      string `json:"status"`
	ActionError string `json:"action_error"`
}

func (m *Run) FromDTO(run dto.RunStatus) Run {
	return Run{
		RunID:       run.RunID.Int64(),
		State:       string(run.State),
		Error:       run.Error,
		Step:        string(run.Step),
		Status:      string(run.Status),
		ActionError: run.ActionError,
	}
}
