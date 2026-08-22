// Command commitcraft drafts a commit message from your staged git diff.
//
// Usage:
//
//	git add <files>
//	commitcraft              # print a suggested message
//	commitcraft --body       # also print a bulleted body
//	commitcraft --apply      # commit immediately with the suggested message
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/zorhehs/commitcraft/internal/generator"
	"github.com/zorhehs/commitcraft/internal/generator/heuristic"
	"github.com/zorhehs/commitcraft/internal/generator/ollama"
	"github.com/zorhehs/commitcraft/internal/gitutil"
)

// version is overwritten at build/release time via -ldflags.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "commitcraft:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("commitcraft", flag.ContinueOnError)
	showBody := fs.Bool("body", false, "also print a bulleted body describing each changed file")
	apply := fs.Bool("apply", false, "run `git commit` with the generated message instead of just printing it")
	showVersion := fs.Bool("version", false, "print the commitcraft version and exit")
	backend := fs.String("backend", "heuristic", "message backend: \"heuristic\" (offline) or \"ollama\" (local LLM via Ollama)")
	model := fs.String("model", "", "Ollama model to use with --backend ollama (default: "+ollama.DefaultModel+")")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "commitcraft drafts a commit message from your staged git diff.")
		fmt.Fprintln(os.Stderr, "\nUsage:")
		fmt.Fprintln(os.Stderr, "  git add <files>")
		fmt.Fprintln(os.Stderr, "  commitcraft [flags]")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVersion {
		fmt.Println("commitcraft", version)
		return nil
	}

	if !gitutil.IsGitRepo() {
		return fmt.Errorf("not a git repository (run this inside a git repo)")
	}

	diff, err := gitutil.Load()
	if err != nil {
		return err
	}

	gen := selectGenerator(*backend, *model)
	msg, err := gen.Generate(diff)
	if err != nil {
		return fmt.Errorf("%s backend: %w", gen.Name(), err)
	}

	if *apply {
		return applyCommit(msg, *showBody)
	}

	printMessage(msg, *showBody)
	return nil
}

// selectGenerator picks a backend based on the --backend flag. If
// "ollama" is requested but no Ollama server is reachable, it prints a
// warning to stderr and falls back to the heuristic backend rather than
// failing outright — commitcraft should always produce *something*.
func selectGenerator(backend, model string) generator.Generator {
	switch backend {
	case "ollama":
		g := ollama.New("", model)
		if g.IsAvailable(context.Background()) {
			return g
		}
		fmt.Fprintf(os.Stderr, "commitcraft: ollama backend requested but no server found at %s — falling back to heuristic\n", g.BaseURL)
		fmt.Fprintln(os.Stderr, "commitcraft: install Ollama from https://ollama.com and run `ollama pull "+ollama.DefaultModel+"` to enable it")
		return heuristic.New()
	default:
		return heuristic.New()
	}
}

func printMessage(msg generator.Message, showBody bool) {
	fmt.Println(msg.Summary)
	if showBody && msg.Body != "" {
		fmt.Println()
		fmt.Println(msg.Body)
	}
}

// applyCommit shows the message, asks for confirmation, then runs
// `git commit` directly so the user never has to copy/paste.
func applyCommit(msg generator.Message, showBody bool) error {
	printMessage(msg, true) // always show the body before committing
	fmt.Print("\nCommit with this message? [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	if len(answer) == 0 || (answer[0] != 'y' && answer[0] != 'Y') {
		fmt.Println("Aborted — nothing committed.")
		return nil
	}

	full := msg.Summary
	if showBody && msg.Body != "" {
		full += "\n\n" + msg.Body
	}

	cmd := exec.Command("git", "commit", "-m", full)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
