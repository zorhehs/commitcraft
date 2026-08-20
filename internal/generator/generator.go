// Package generator defines the pluggable interface that every commit
// message backend implements. Today only the heuristic (offline, no LLM)
// backend exists. A future Ollama-backed generator can be dropped in
// without changing anything upstream, because callers only ever depend
// on this interface.
package generator

import "github.com/zorhehs/commitcraft/internal/gitutil"

// Message is a generated commit message, split into a short summary
// (the conventional-commit subject line) and an optional longer body.
type Message struct {
	Summary string // e.g. "feat(auth): add password reset flow"
	Body    string // optional wrapped explanation, may be empty
}

// Generator produces a commit message from a staged diff.
type Generator interface {
	// Name identifies the backend for logging/CLI output, e.g. "heuristic".
	Name() string
	// Generate returns a commit message for the given staged diff.
	Generate(diff *gitutil.StagedDiff) (Message, error)
}
