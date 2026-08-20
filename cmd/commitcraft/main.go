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
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/zorhehs/commitcraft/internal/generator"
	"github.com/zorhehs/commitcraft/internal/generator/heuristic"
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

	gen := selectGenerator()
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

// selectGenerator picks a backend. Today there is only the heuristic
// (offline) backend; this is the single seam where an Ollama-backed
// generator gets added later without touching the rest of main.
func selectGenerator() generator.Generator {
	return heuristic.New()
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
