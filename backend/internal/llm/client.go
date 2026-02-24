package llm

import "context"

// TaskClassification represents the result of an LLM classifying a task.
type TaskClassification struct {
	Tags     []string `json:"tags"`
	Priority string   `json:"priority"` // "high", "medium", "low"
	Category string   `json:"category"` // "bug", "feature", "improvement", "research"
	Summary  string   `json:"summary"`  // One-line summary
}

type LLMClient interface {
	ClassifyTask(ctx context.Context, title string, description string) (*TaskClassification, error)
}
