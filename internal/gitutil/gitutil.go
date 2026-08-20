// Package gitutil wraps the git CLI to read information about the
// currently staged changes. It shells out to `git` rather than parsing
// .git internals directly, which keeps this package small and correct
// across git versions.
package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// FileChange describes a single file in the staged diff.
type FileChange struct {
	Path      string // path relative to repo root
	Status    string // "A" (added), "M" (modified), "D" (deleted), "R" (renamed), etc.
	Additions int
	Deletions int
}

// StagedDiff holds everything we know about the current staging area.
type StagedDiff struct {
	Files   []FileChange
	RawDiff string // full unified diff text, used as LLM context later
}

// IsGitRepo reports whether the current working directory is inside a git
// repository.
func IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

// StagedFiles returns the list of files currently staged for commit, along
// with per-file line-change counts, using `git diff --cached --numstat`.
func StagedFiles() ([]FileChange, error) {
	numstat, err := runGit("diff", "--cached", "--numstat")
	if err != nil {
		return nil, err
	}
	statuses, err := statusMap()
	if err != nil {
		return nil, err
	}

	var files []FileChange
	for _, line := range splitNonEmptyLines(numstat) {
		// Format: "<added>\t<deleted>\t<path>"
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		add, _ := strconv.Atoi(parts[0]) // "-" for binary files -> 0
		del, _ := strconv.Atoi(parts[1])
		path := parts[2]
		files = append(files, FileChange{
			Path:      path,
			Status:    statuses[path],
			Additions: add,
			Deletions: del,
		})
	}
	return files, nil
}

// RawDiff returns the full unified diff of staged changes.
func RawDiff() (string, error) {
	return runGit("diff", "--cached")
}

// Load gathers the full staged-diff picture in one call.
func Load() (*StagedDiff, error) {
	files, err := StagedFiles()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no staged changes found — stage something with `git add` first")
	}
	raw, err := RawDiff()
	if err != nil {
		return nil, err
	}
	return &StagedDiff{Files: files, RawDiff: raw}, nil
}

// statusMap returns path -> one-letter status ("A", "M", "D", "R", ...)
// from `git diff --cached --name-status`.
func statusMap() (map[string]string, error) {
	out, err := runGit("diff", "--cached", "--name-status")
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, line := range splitNonEmptyLines(out) {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		status := string(parts[0][0]) // handles "R100" -> "R"
		path := parts[len(parts)-1]   // renames: "old\tnew", take new
		m[path] = status
	}
	return m, nil
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
