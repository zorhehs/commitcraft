# commitcraft

Drafts a commit message from your staged `git diff` — no API key, no
network call, no setup. Point it at a repo, stage some files, run it.

```
$ git add internal/auth/login.go
$ commitcraft
feat(auth): add internal/auth/login.go
```

## Why

Writing a good commit message for a small change is annoying enough that
people skip it and write `wip` or `fix stuff`. commitcraft looks at *what
actually changed* — which files, how many lines, added vs. modified vs.
deleted — and drafts a [Conventional Commits](https://www.conventionalcommits.org/)
style message in under a second, fully offline.

## Install

Requires Go 1.22+.

```
go install github.com/zorhehs/commitcraft/cmd/commitcraft@latest
```

Or build from source:

```
git clone https://github.com/zorhehs/commitcraft
cd commitcraft
go build -o commitcraft ./cmd/commitcraft
```

## Usage

```
git add <files>
commitcraft                        # heuristic backend (default), fully offline
commitcraft --body                 # also print a bulleted summary of each file
commitcraft --apply                # show the message, confirm, then `git commit` directly
commitcraft --backend ollama       # use a local LLM via Ollama instead of heuristics
commitcraft --backend ollama --model phi3   # pick a different pulled Ollama model
commitcraft --version
```

If `--backend ollama` is passed but no Ollama server is reachable at
`localhost:11434`, commitcraft prints a warning and falls back to the
heuristic backend automatically — it never just fails outright.

## How it works

commitcraft never sends your code anywhere by default. It has two
backends behind a small shared interface
([`internal/generator`](internal/generator)):

```go
type Generator interface {
    Name() string
    Generate(diff *gitutil.StagedDiff) (Message, error)
}
```

**heuristic** (default) — reads file paths, git statuses
(added/modified/deleted), and diff size, and applies a handful of rules
to pick a Conventional Commits type (`feat`, `fix`, `docs`, `test`,
`ci`, `chore`) and scope. Instant, dependency-free, works with zero
setup. See [`internal/generator/heuristic`](internal/generator/heuristic)
for the exact rules.

**ollama** (opt-in via `--backend ollama`) — sends the staged diff to a
model running locally through [Ollama](https://ollama.com) and asks it
to draft a Conventional Commits message. Requires Ollama installed and
a model pulled (`ollama pull llama3.2:1b`), but still runs entirely on
your machine — no API key, no network egress. If Ollama isn't running,
commitcraft falls back to heuristic automatically. See
[`internal/generator/ollama`](internal/generator/ollama).

Because both implement the same interface, adding a third backend later
(say, a hosted API) is a matter of writing one more small package —
`main.go`, `gitutil`, and the CLI flags don't need to change.

## Roadmap

- [x] `v0.1` — heuristic backend, `--body`, `--apply`
- [x] `v0.2` — optional Ollama backend (`--backend ollama`, `--model`), graceful fallback when Ollama isn't running
- [ ] `v0.3` — `--pr` flag to draft a fuller pull request description
- [ ] `v1.0` — tagged release with prebuilt binaries via GoReleaser

Known limitations, tracked as issues:
- Heuristic scope detection can be noisy across unrelated directories
- Ollama output quality depends heavily on the chosen model — small
  models (e.g. `llama3.2:1b`) are fast but don't always follow the
  formatting instructions precisely

## Contributing

Issues and PRs welcome. The codebase is small on purpose:

```
cmd/commitcraft/          CLI entrypoint (stdlib flag package, no framework)
internal/gitutil/         shells out to `git` to read staged changes
internal/generator/       backend interface + Message type
internal/generator/heuristic/   the default offline backend
```

Run tests with `go test ./...` before opening a PR.

## License

[MIT](LICENSE)
