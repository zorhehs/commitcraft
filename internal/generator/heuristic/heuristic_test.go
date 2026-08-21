package heuristic

import (
	"strings"
	"testing"

	"github.com/zorhehs/commitcraft/internal/gitutil"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name       string
		files      []gitutil.FileChange
		wantPrefix string // subject must start with this
		wantErr    bool
	}{
		{
			name:    "no files is an error",
			files:   nil,
			wantErr: true,
		},
		{
			name: "single new file is feat",
			files: []gitutil.FileChange{
				{Path: "internal/auth/login.go", Status: "A", Additions: 40},
			},
			wantPrefix: "feat(auth): add internal/auth/login.go",
		},
		{
			name: "single modified file is fix",
			files: []gitutil.FileChange{
				{Path: "internal/auth/login.go", Status: "M", Additions: 3, Deletions: 1},
			},
			wantPrefix: "fix(auth): update internal/auth/login.go",
		},
		{
			name: "all test files is test type",
			files: []gitutil.FileChange{
				{Path: "internal/auth/login_test.go", Status: "M"},
				{Path: "internal/auth/session_test.go", Status: "A"},
			},
			wantPrefix: "test(auth):",
		},
		{
			name: "all markdown files is docs type",
			files: []gitutil.FileChange{
				{Path: "README.md", Status: "M"},
				{Path: "docs/setup.md", Status: "M"},
			},
			wantPrefix: "docs:",
		},
		{
			name: "ci workflow files get ci type",
			files: []gitutil.FileChange{
				{Path: ".github/workflows/ci.yml", Status: "A"},
			},
			wantPrefix: "ci",
		},
		{
			name: "deleted only file with no additions is chore",
			files: []gitutil.FileChange{
				{Path: "internal/legacy/old.go", Status: "D"},
			},
			wantPrefix: "chore(legacy): remove internal/legacy/old.go",
		},
		{
			name: "multiple files summarizes counts",
			files: []gitutil.FileChange{
				{Path: "internal/a/a.go", Status: "A"},
				{Path: "internal/a/b.go", Status: "M"},
				{Path: "internal/a/c.go", Status: "D"},
			},
			wantPrefix: "feat(a): add 1, update 1, remove 1 files",
		},
		{
			name: "small modification stays fix",
			files: []gitutil.FileChange{
				{Path: "src/app.py", Status: "M", Additions: 10, Deletions: 5},
			},
			wantPrefix: "fix",
		},
		{
			name: "large modification becomes feat",
			files: []gitutil.FileChange{
				{Path: "src/app.py", Status: "M", Additions: 200, Deletions: 24},
			},
			wantPrefix: "feat(src): update src/app.py",
		},
		{
			name: "modification exactly at threshold becomes feat",
			files: []gitutil.FileChange{
				{Path: "src/app.py", Status: "M", Additions: 40, Deletions: 10},
			},
			wantPrefix: "feat",
		},
		{
			name: "modification just under threshold stays fix",
			files: []gitutil.FileChange{
				{Path: "src/app.py", Status: "M", Additions: 30, Deletions: 19},
			},
			wantPrefix: "fix",
		},
	}

	g := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := &gitutil.StagedDiff{Files: tt.files}
			msg, err := g.Generate(diff)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (message: %q)", msg.Summary)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(msg.Summary, tt.wantPrefix) {
				t.Errorf("subject = %q, want prefix %q", msg.Summary, tt.wantPrefix)
			}
		})
	}
}

func TestGenerateBodyListsAllFiles(t *testing.T) {
	g := New()
	diff := &gitutil.StagedDiff{Files: []gitutil.FileChange{
		{Path: "b.go", Status: "M", Additions: 2, Deletions: 1},
		{Path: "a.go", Status: "A", Additions: 10},
	}}

	msg, err := g.Generate(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Body should list both files, sorted, each on its own bullet line.
	lines := strings.Split(msg.Body, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 body lines, got %d: %q", len(lines), msg.Body)
	}
	if !strings.Contains(lines[0], "a.go") {
		t.Errorf("expected a.go to sort first, got: %q", lines[0])
	}
	if !strings.Contains(lines[1], "b.go") {
		t.Errorf("expected b.go second, got: %q", lines[1])
	}
}

func TestName(t *testing.T) {
	if got := New().Name(); got != "heuristic" {
		t.Errorf("Name() = %q, want %q", got, "heuristic")
	}
}
