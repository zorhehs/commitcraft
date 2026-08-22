package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zorhehs/commitcraft/internal/gitutil"
)

func sampleDiff() *gitutil.StagedDiff {
	return &gitutil.StagedDiff{
		Files: []gitutil.FileChange{
			{Path: "internal/auth/login.go", Status: "M", Additions: 12, Deletions: 3},
		},
		RawDiff: "diff --git a/internal/auth/login.go b/internal/auth/login.go\n+func Logout() {}\n",
	}
}

func TestGenerate_ParsesModelResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Model != "llama3.2:1b" {
			t.Errorf("model = %q, want llama3.2:1b", req.Model)
		}
		if req.Stream {
			t.Error("expected Stream: false")
		}

		resp := generateResponse{
			Response: "feat(auth): add logout support\n\n- add Logout function\n- update login flow",
			Done:     true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	g := New(server.URL, "")
	msg, err := g.Generate(sampleDiff())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Summary != "feat(auth): add logout support" {
		t.Errorf("Summary = %q, want %q", msg.Summary, "feat(auth): add logout support")
	}
	wantBody := "- add Logout function\n- update login flow"
	if msg.Body != wantBody {
		t.Errorf("Body = %q, want %q", msg.Body, wantBody)
	}
}

func TestGenerate_NoStagedFiles(t *testing.T) {
	g := New("http://unused", "")
	_, err := g.Generate(&gitutil.StagedDiff{})
	if err == nil {
		t.Fatal("expected error for empty diff, got none")
	}
}

func TestGenerate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	g := New(server.URL, "")
	_, err := g.Generate(sampleDiff())
	if err == nil {
		t.Fatal("expected error for 500 response, got none")
	}
}

func TestGenerate_Unreachable(t *testing.T) {
	// Port 1 is reserved and nothing will ever be listening there, so
	// this simulates Ollama not running without relying on timing.
	g := New("http://127.0.0.1:1", "")
	_, err := g.Generate(sampleDiff())
	if err == nil {
		t.Fatal("expected error when server is unreachable, got none")
	}
}

func TestGenerate_EmptyModelResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(generateResponse{Response: "   \n\n  ", Done: true})
	}))
	defer server.Close()

	g := New(server.URL, "")
	_, err := g.Generate(sampleDiff())
	if err == nil {
		t.Fatal("expected error for empty model response, got none")
	}
}

func TestIsAvailable_ServerUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	g := New(server.URL, "")
	if !g.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable to be true when server responds 200")
	}
}

func TestIsAvailable_ServerDown(t *testing.T) {
	g := New("http://127.0.0.1:1", "")
	if g.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable to be false when nothing is listening")
	}
}

func TestNew_Defaults(t *testing.T) {
	g := New("", "")
	if g.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want default %q", g.BaseURL, DefaultBaseURL)
	}
	if g.Model != DefaultModel {
		t.Errorf("Model = %q, want default %q", g.Model, DefaultModel)
	}
}

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantSummary string
		wantBody    string
	}{
		{
			name:        "summary and body",
			raw:         "fix: bug\n\n- detail one\n- detail two",
			wantSummary: "fix: bug",
			wantBody:    "- detail one\n- detail two",
		},
		{
			name:        "summary only, no body",
			raw:         "fix: bug",
			wantSummary: "fix: bug",
			wantBody:    "",
		},
		{
			name:        "extra blank lines before body are skipped",
			raw:         "fix: bug\n\n\n- detail",
			wantSummary: "fix: bug",
			wantBody:    "- detail",
		},
		{
			name:        "empty input",
			raw:         "   ",
			wantSummary: "",
			wantBody:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := parseMessage(tt.raw)
			if msg.Summary != tt.wantSummary {
				t.Errorf("Summary = %q, want %q", msg.Summary, tt.wantSummary)
			}
			if msg.Body != tt.wantBody {
				t.Errorf("Body = %q, want %q", msg.Body, tt.wantBody)
			}
		})
	}
}
