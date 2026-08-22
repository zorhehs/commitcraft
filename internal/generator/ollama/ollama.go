// Package ollama implements a commit-message Generator backed by a local
// Ollama server (https://ollama.com). It talks to Ollama's HTTP API over
// localhost, so no API key and no network egress are required — the
// model runs entirely on the user's machine.
//
// This backend is optional: it satisfies the same generator.Generator
// interface as the heuristic backend, so callers (main.go) can select it
// with a flag and fall back to heuristic if Ollama isn't reachable.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zorhehs/commitcraft/internal/generator"
	"github.com/zorhehs/commitcraft/internal/gitutil"
)

const (
	// DefaultBaseURL is where Ollama listens by default on the local
	// machine after `ollama serve` (or the background app) starts.
	DefaultBaseURL = "http://localhost:11434"

	// DefaultModel is a small, fast model that's a reasonable default
	// for a short task like drafting a commit message. Users can pick
	// something else with --model as long as they've pulled it locally
	// (`ollama pull <model>`).
	DefaultModel = "llama3.2:1b"

	// maxDiffChars bounds how much of the raw diff we send in the
	// prompt. Commit messages don't need the entire diff to be useful,
	// and keeping the prompt small keeps small local models fast.
	maxDiffChars = 6000

	requestTimeout = 20 * time.Second
	pingTimeout    = 2 * time.Second
)

// Generator calls a local Ollama server to draft a commit message.
type Generator struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

// New returns an ollama Generator. Pass "" for baseURL or model to use
// the package defaults.
func New(baseURL, model string) *Generator {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if model == "" {
		model = DefaultModel
	}
	return &Generator{
		BaseURL: baseURL,
		Model:   model,
		Client:  &http.Client{Timeout: requestTimeout},
	}
}

func (g *Generator) Name() string { return "ollama (" + g.Model + ")" }

// IsAvailable reports whether an Ollama server is reachable at BaseURL.
// Callers should check this before selecting the ollama backend, so they
// can fall back to heuristic instead of failing outright.
func (g *Generator) IsAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.BaseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// generateRequest mirrors the subset of Ollama's /api/generate request
// body that we use. See https://github.com/ollama/ollama/blob/main/docs/api.md
type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// generateResponse mirrors the subset of Ollama's /api/generate response
// body that we use (with Stream: false, Ollama returns one JSON object).
type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate drafts a commit message by sending the staged diff to the
// local Ollama model and parsing its response.
func (g *Generator) Generate(diff *gitutil.StagedDiff) (generator.Message, error) {
	if diff == nil || len(diff.Files) == 0 {
		return generator.Message{}, fmt.Errorf("ollama: no staged files to describe")
	}

	prompt := buildPrompt(diff)

	reqBody, err := json.Marshal(generateRequest{
		Model:  g.Model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return generator.Message{}, fmt.Errorf("ollama: encoding request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return generator.Message{}, fmt.Errorf("ollama: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.Client.Do(req)
	if err != nil {
		return generator.Message{}, fmt.Errorf("ollama: request failed (is `ollama serve` running at %s?): %w", g.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return generator.Message{}, fmt.Errorf("ollama: server returned status %d", resp.StatusCode)
	}

	var parsed generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return generator.Message{}, fmt.Errorf("ollama: decoding response: %w", err)
	}

	msg := parseMessage(parsed.Response)
	if msg.Summary == "" {
		return generator.Message{}, fmt.Errorf("ollama: model returned an empty message")
	}
	return msg, nil
}

// buildPrompt turns a staged diff into an instruction prompt. The diff is
// truncated to maxDiffChars to keep small local models fast and to avoid
// exceeding typical context windows.
func buildPrompt(diff *gitutil.StagedDiff) string {
	rawDiff := diff.RawDiff
	truncated := false
	if len(rawDiff) > maxDiffChars {
		rawDiff = rawDiff[:maxDiffChars]
		truncated = true
	}

	var b strings.Builder
	b.WriteString("You are an expert software engineer writing a git commit message.\n")
	b.WriteString("Given the staged diff below, respond with ONLY the commit message — no preamble, no explanation, no markdown fences.\n\n")
	b.WriteString("Format exactly like this:\n")
	b.WriteString("<type>(<scope>): <short summary, imperative mood, under 72 chars>\n\n")
	b.WriteString("- <bullet point describing a change>\n")
	b.WriteString("- <bullet point describing another change>\n\n")
	b.WriteString("Use a Conventional Commits type: feat, fix, docs, test, refactor, chore, or ci.\n")
	b.WriteString("Omit the scope parentheses if there's no clear single scope.\n\n")
	b.WriteString("Diff:\n")
	b.WriteString(rawDiff)
	if truncated {
		b.WriteString("\n[diff truncated]\n")
	}
	return b.String()
}

// parseMessage splits a raw model response into a Message: the first
// non-empty line is the summary, everything after the first blank line
// is the body.
func parseMessage(raw string) generator.Message {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return generator.Message{}
	}

	lines := strings.Split(raw, "\n")
	summary := strings.TrimSpace(lines[0])

	var bodyLines []string
	// Skip the summary line and any immediately-following blank lines,
	// then keep everything else as the body.
	i := 1
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	bodyLines = lines[i:]

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	return generator.Message{Summary: summary, Body: body}
}
