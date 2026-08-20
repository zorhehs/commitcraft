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
commitcraft              # print a suggested commit message
commitcraft --body       # also print a bulleted summary of each file
commitcraft --apply      # show the message, confirm, then `git commit` directly
commitcraft --version
```

## How it works

commitcraft never sends your code anywhere. The default (and currently
only) backend is **heuristic**: it reads file paths, git statuses
(added/modified/deleted), and diff size, and applies a handful of rules
to pick a Conventional Commits type (`feat`, `fix`, `docs`, `test`, `ci`,
`chore`) and scope. See [`internal/generator/heuristic`](internal/generator/heuristic)
for the exact rules.

The message generator is a small interface
([`internal/generator`](internal/generator)):

```go
type Generator interface {
    Name() string
    Generate(diff *gitutil.StagedDiff) (Message, error)
}
```

This is intentional: a future backend that calls a **local LLM via
[Ollama](https://ollama.com)** for higher-quality messages can be added
as a second implementation of this interface without touching `main.go`,
`gitutil`, or the CLI flags. See [Roadmap](#roadmap).

## Roadmap

- [x] `v0.1` — heuristic backend, `--body`, `--apply`
- [ ] `v0.2` — optional Ollama backend (`--backend ollama`), config file for model choice
- [ ] `v0.3` — `--pr` flag to draft a fuller pull request description
- [ ] `v1.0` — tagged release with prebuilt binaries via GoReleaser

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
