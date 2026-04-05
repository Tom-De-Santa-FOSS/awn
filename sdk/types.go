package sdk

import "github.com/tom/awn"

// Session represents a created terminal session.
type Session struct {
	ID string `json:"id"`
}

// Screen represents a screenshot response.
type Screen struct {
	Rows     int            `json:"rows"`
	Cols     int            `json:"cols"`
	Hash     string         `json:"hash"`
	Lines    []string       `json:"lines,omitempty"`
	History  []string       `json:"history,omitempty"`
	Cursor   awn.Position   `json:"cursor"`
	Elements []awn.Element  `json:"elements,omitempty"`
	State    string         `json:"state,omitempty"`
}

// DetectResult represents a detect response.
type DetectResult struct {
	Elements []awn.DetectElement  `json:"elements"`
	Tree     []awn.DetectTreeNode `json:"tree,omitempty"`
	Viewport awn.Rect             `json:"viewport,omitempty"`
	Scrolled bool                 `json:"scrolled,omitempty"`
}

// SessionInfo represents details about a session.
type SessionInfo struct {
	ID      string `json:"id"`
	Command string `json:"command,omitempty"`
	Current bool   `json:"current,omitempty"`
}

// ListResponse wraps a list of session IDs.
type ListResponse struct {
	Sessions []string `json:"sessions"`
}

// PipelineResult holds the result of a pipeline execution.
type PipelineResult struct {
	Results []StepResult `json:"results"`
}

// StepResult is the result of a single pipeline step.
type StepResult struct {
	Step   int     `json:"step"`
	Error  string  `json:"error,omitempty"`
	Screen *Screen `json:"screen,omitempty"`
}

// ExecResult wraps an exec response.
type ExecResult struct {
	Screen *Screen `json:"screen"`
}

// PingResponse wraps a ping response.
type PingResponse struct {
	Status string `json:"status"`
}
