# AGENTS.md

Guidance for AI agents and contributors working in this repository.

## Non-negotiable: never expose secrets

This is a **public** repository. Treat every commit as world-readable.

- NEVER commit API keys, tokens, passwords, private keys, cookies, connection
  strings, or any credential — not in code, config, tests, fixtures, docs,
  commit messages, or comments.
- NEVER print secret values into logs, terminal output, error messages, or the TUI.
- Real credentials belong in environment variables or an untracked local file
  (for example `.env.local`), never in a tracked file.
- A config template that needs a secret MUST use an obvious placeholder
  (`${OPENAI_API_KEY}`, `REPLACE_ME`), and the runtime MUST read the real value
  from the environment.
- `kit export` MUST sanitize every captured config before it leaves the machine:
  redact or placeholder any field whose key matches known secret patterns
  (`api_key`, `apikey`, `token`, `authorization`, `cookie`, `password`, `secret`,
  `*_KEY`, `*_TOKEN`, `*_SECRET`, private key blocks).
- If a secret is ever committed, rotate it immediately. Rewriting history does
  not undo the leak.

### Pre-commit secret check

Before every commit, inspect the staged diff and confirm it is clean:

    git diff --cached

Read it. If anything looks like a credential, remove it before committing.

## What this project is

`kit` is a cross-platform TUI (Go + Bubbletea) that detects, exports, and installs
a terminal/dev kit — Zellij + Alacritty + nvim (LazyVim) + starship + mise +
opencode — on Linux, WSL2, and (planned) Windows native.

## Layout

    cmd/kit/            entrypoint: TUI + CLI subcommands
    internal/manifest/  kit.yaml types, loader, validation
    internal/inventory/ OS/arch/distro/WSL + tool detection
    internal/target/    target strategies (linux|wsl|windows) + path resolution
    internal/portable/  home-path normalization and rendering
    internal/bundle/    export: capture, sanitize, rewrite, zip
    internal/restore/   apply a bundle's configs to a target
    internal/install/   install plan and execution
    internal/tui/       Bubbletea models, screens, styles
    kits/               embedded default manifest + seed configs (embed.FS)
    odd/tasks/          feature task documents

## Build, test, run

    go build ./...
    go vet ./...
    go test ./...
    go run ./cmd/kit          # launch the TUI
    go run ./cmd/kit detect   # environment + component status

## Conventions

- Go code is `gofmt`-clean; wrap errors with `%w` and context.
- Code, comments, docs, and commit messages are written in **English**.
- Use repository-relative paths in docs and memory — never hardcode a
  machine-specific absolute path.
- Targets are `linux`, `wsl`, and `windows`. Every manifest change must validate
  for all three.
- Keep the binary self-contained: assets ship through `kits/embed.go` (embed.FS),
  never as loose files read from disk at runtime.

## Commits

- Conventional Commits.
- One work unit per commit: code + tests + docs travel together.
- Never add AI attribution or `Co-Authored-By` trailers.

## Task tracking

- Feature work lives in `odd/tasks/<feature>.md` with checkbox task IDs.
- Check a task off only after its outcome and checks were actually observed.
