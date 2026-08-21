// Package heuristic implements a commit-message generator that needs no
// LLM at all: it inspects file paths, statuses, and diff size to draft a
// Conventional Commits-style message. It is the default backend so that
// commitcraft is fully usable offline, with zero setup, on first install.
package heuristic

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/zorhehs/commitcraft/internal/generator"
	"github.com/zorhehs/commitcraft/internal/gitutil"
)

// Generator is the heuristic, dependency-free backend.
type Generator struct{}

// New returns a ready-to-use heuristic Generator.
func New() *Generator {
	return &Generator{}
}

func (g *Generator) Name() string { return "heuristic" }

// largeChangeThreshold is the total number of changed lines (additions +
// deletions) across modified-only files above which we treat the change
// as substantial enough to be "feat" rather than "fix". A one-line typo
// fix and a 200-line rewrite should not both be labeled "fix".
const largeChangeThreshold = 50

// Generate drafts a Conventional Commits message from the staged diff
// using only path/status heuristics — no network, no LLM.
func (g *Generator) Generate(diff *gitutil.StagedDiff) (generator.Message, error) {
	if diff == nil || len(diff.Files) == 0 {
		return generator.Message{}, fmt.Errorf("heuristic: no staged files to describe")
	}

	commitType := inferType(diff.Files)
	scope := inferScope(diff.Files)
	subject := buildSubject(commitType, scope, diff.Files)
	body := buildBody(diff.Files)

	return generator.Message{Summary: subject, Body: body}, nil
}

// inferType guesses a Conventional Commits type from the set of changed
// files. Rules are checked in priority order; the first match wins.
func inferType(files []gitutil.FileChange) string {
	var (
		allTest            = true
		allDocs            = true
		allCI              = true
		hasNewFile         = false
		hasDeleted         = false
		totalModifiedLines = 0
	)

	for _, f := range files {
		lower := strings.ToLower(f.Path)

		if !isTestFile(lower) {
			allTest = false
		}
		if !isDocFile(lower) {
			allDocs = false
		}
		if !isCIFile(lower) {
			allCI = false
		}
		switch f.Status {
		case "A":
			hasNewFile = true
		case "D":
			hasDeleted = true
		case "M":
			totalModifiedLines += f.Additions + f.Deletions
		}
	}

	switch {
	case allTest:
		return "test"
	case allDocs:
		return "docs"
	case allCI:
		return "ci"
	case hasDeleted && !hasNewFile:
		return "chore"
	case hasNewFile:
		return "feat"
	case totalModifiedLines >= largeChangeThreshold:
		return "feat"
	default:
		return "fix"
	}
}

func isTestFile(lowerPath string) bool {
	return strings.Contains(lowerPath, "_test.go") ||
		strings.Contains(lowerPath, "/test") ||
		strings.Contains(lowerPath, "/tests/") ||
		strings.HasPrefix(lowerPath, "test_") ||
		strings.Contains(lowerPath, ".test.")
}

func isDocFile(lowerPath string) bool {
	return strings.HasSuffix(lowerPath, ".md") ||
		strings.HasSuffix(lowerPath, ".rst") ||
		strings.HasPrefix(lowerPath, "docs/")
}

func isCIFile(lowerPath string) bool {
	base := path.Base(lowerPath)
	return strings.Contains(lowerPath, ".github/workflows") ||
		base == "makefile" ||
		(strings.HasSuffix(lowerPath, ".yml") && strings.Contains(lowerPath, "ci"))
}

// inferScope picks a short scope name from the deepest common directory
// shared by all changed files. Returns "" if files are scattered across
// unrelated top-level directories.
func inferScope(files []gitutil.FileChange) string {
	if len(files) == 0 {
		return ""
	}
	common := path.Dir(files[0].Path)
	for _, f := range files[1:] {
		common = commonDir(common, path.Dir(f.Path))
	}
	if common == "." || common == "" {
		return ""
	}
	// Use only the last path segment as scope, e.g. "internal/auth" -> "auth".
	return path.Base(common)
}

func commonDir(a, b string) string {
	aParts := strings.Split(a, "/")
	bParts := strings.Split(b, "/")
	var common []string
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		if aParts[i] != bParts[i] {
			break
		}
		common = append(common, aParts[i])
	}
	if len(common) == 0 {
		return "."
	}
	return strings.Join(common, "/")
}

func buildSubject(commitType, scope string, files []gitutil.FileChange) string {
	desc := describeChange(files)
	if scope != "" && scope != "." {
		return fmt.Sprintf("%s(%s): %s", commitType, scope, desc)
	}
	return fmt.Sprintf("%s: %s", commitType, desc)
}

// describeChange produces the short human-readable part of the subject
// line, e.g. "add gitutil package" or "update 3 files in internal/".
func describeChange(files []gitutil.FileChange) string {
	if len(files) == 1 {
		f := files[0]
		verb := verbForStatus(f.Status)
		return fmt.Sprintf("%s %s", verb, f.Path)
	}

	added, modified, deleted := 0, 0, 0
	for _, f := range files {
		switch f.Status {
		case "A":
			added++
		case "D":
			deleted++
		default:
			modified++
		}
	}

	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("add %d", added))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("update %d", modified))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("remove %d", deleted))
	}
	return fmt.Sprintf("%s files", strings.Join(parts, ", "))
}

func verbForStatus(status string) string {
	switch status {
	case "A":
		return "add"
	case "D":
		return "remove"
	case "R":
		return "rename"
	default:
		return "update"
	}
}

// buildBody lists each changed file with its line-delta as bullet points,
// sorted by path for stable output.
func buildBody(files []gitutil.FileChange) string {
	sorted := make([]gitutil.FileChange, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	var b strings.Builder
	for _, f := range sorted {
		fmt.Fprintf(&b, "- %s %s (+%d/-%d)\n", verbForStatus(f.Status), f.Path, f.Additions, f.Deletions)
	}
	return strings.TrimRight(b.String(), "\n")
}
