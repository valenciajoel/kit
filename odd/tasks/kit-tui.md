# Feature: kit-tui — Cross-platform terminal kit TUI

## Objective
Ship a single-binary Go TUI (`kit`) that detects, exports, and installs the user's
terminal/dev kit — Zellij + Alacritty + nvim (LazyVim) + starship + mise + opencode —
on **Linux**, **WSL2**, and (later) **Windows native**, so a dual-boot machine can be
reprovisioned from one portable bundle.

## Problem
- The kit is Linux-only by construction: shell layer is Omakub + bash, and
  `alacritty.toml` imports a Linux-only path (`~/.config/omakub/current/...`).
- Reinstalling on a fresh OS (or the Windows side of a dual boot) is manual and
  error-prone.
- `opencode.json` likely holds API keys — a naive "export everything" leaks secrets.
- Language runtimes are managed by `mise`, not standalone installers.

## Scope
**In**
- Target model: `linux`, `wsl`, `windows` (interface + detection for all three;
  implementation focus on `linux`/`wsl` first).
- Manifest-driven component model (`kit.yaml`).
- Inventory: detect OS, arch, distro, WSL, home/config paths, and per-tool presence.
- Export: capture configs into a portable bundle with path templating and secret
  sanitization.
- Install: per-target installers (apt, mise, npm, script, winget) with dry-run.
- TUI: dashboard / inventory / export / install screens.

**Out (for now)**
- Windows-native installer bodies (only the strategy interface + detection).
- Full disk/dotfile backup (git config, SSH/GPG keys) — designed but deferred.
- Any GUI.

## Constraints
- Go 1.26.3; Bubbletea + Lipgloss + yaml.v3.
- No secrets ever leave the machine in a bundle unredacted.
- Config paths must be templated per target (`~`, `%APPDATA%`).
- WSL target = Linux layer inside WSL + thin Windows-host layer (Alacritty.exe,
  Nerd Font, Windows Terminal) applied later.

## Architecture
```
cmd/kit/main.go            entrypoint: CLI subcommands + TUI
internal/manifest/         types + loader/validator for kit.yaml
internal/inventory/        OS/arch/distro/WSL + tool detection
internal/target/           target strategy (linux|wsl|windows) + path resolution
internal/bundle/           export: copy, template, sanitize, archive
internal/install/          per-target installers (strategy + methods)
internal/tui/              Bubbletea app + screens + styles
kits/                      embedded default manifest + seed configs (embed.FS)
odd/tasks/kit-tui.md       this document
```

## Manifest schema (v1)
```yaml
version: 1
components:
  - id: zellij
    name: Zellij
    kind: terminal-multiplexer
    configs:
      - src: "~/.config/zellij/config.kdl"
        dest:
          linux: "~/.config/zellij/config.kdl"
          wsl:   "~/.config/zellij/config.kdl"
          windows: "%APPDATA%/zellij/config.kdl"
    install:
      linux:   { method: apt,  pkg: zellij }
      wsl:     { method: script, url: "..." }
      windows: { method: winget, pkg: "Zellij.Zellij" }
    optional: false
```

## Checklist
- [x] **T1** Scaffold: go.mod, package layout, embedded `kits` FS. — `go build ./...` OK
- [x] **T2** `internal/manifest`: types, YAML loader, validation, lookup by id. — validates 10 components
- [x] **T3** `internal/inventory`: OS/arch/distro/WSL detection + tool presence/version. — detects Ubuntu 24.04, bash, WSL=false
- [x] **T4** `internal/target`: three targets, config-root + path expansion (`~`, `%APPDATA%`). — `linux` target resolved
- [x] **T5** `internal/tui`: Bubbletea dashboard + inventory screen, rescan keybind. — compiles; interactive run pending manual check
- [x] **T6** `cmd/kit`: CLI wiring (`detect`, `export`, `install`) + TUI default. — `kit detect` prints all 10 components
- [x] **T7** `internal/bundle`: capture configs, resolve paths, secret sanitization, zip. — 103 files, 1 real secret redacted, paths relativized to `~`
- [x] **T8** `internal/install`: strategy interface + apt/mise/npm/script methods + dry-run. — plan builds 9 run + 1 skip for linux
- [x] **T9** `internal/portable`: home-path normalization/render + manifest `rewrites`
      (fixes the Omakub alacritty import) + seed configs in `kits/configs/`.
- [x] **T11** `internal/restore` + `kit restore`: apply a bundle's configs to a target,
      rendering `~` per destination machine; seeds fill gaps. (added when the missing
      import/restore step was identified)
- [x] **T10** Tests (sanitize, install plan, portable round-trip, export→restore), README,
      Makefile with linux/windows builds.

## Route / trigger evidence
- Writer trigger (2+ non-trivial files) would normally delegate. This runtime exposes
  **only SDD phase agents**; using one outside a selected SDD route is a contract
  violation, so slices are implemented inline and verified with `go build`/`go test`.
- Long-session backstop: checkpoint to Engram after each accepted slice.

## Acceptance criteria
- `go build ./...` and `go test ./...` pass.
- `kit detect` prints OS, arch, distro, WSL flag, config root, and per-component status.
- TUI lists every manifest component with present/absent + version, and rescan works.
- `kit export --dry-run` lists what would be captured and flags secrets, without writing.
- No bundle path ever contains an unredacted secret.

## Checks
- `go vet ./...`
- `go build ./...`
- `go test ./...`
- manual: `go run ./cmd/kit detect`

## Delivery
- Strategy: `ask-on-risk` (default). Per-task work-unit commits once the repo is
  initialized (pending user confirmation: repo is not yet git).
- Size heuristic: ~400 authored lines per task (advisory only).

## Progress log
- 2026-10-05: Feature document created. Decisions locked: Go + Bubbletea; dual-target
  architecture with WSL2 implemented first; native Windows later. Kit inventoried
  (Ubuntu 24.04, Omakub, bash, mise, Alacritty+Zellij, nvim/LazyVim, starship, opencode).
- 2026-10-05: Slice 1 (T1–T6) implemented inline. `go build`, `go vet`, `go run ./cmd/kit detect`
  all pass. Detection reported 9/10 components present on the host.
  **Discovered**: `opencode` resolves to an `npx` cache path, i.e. it is not installed
  globally — T8's npm method must use `npm i -g opencode-ai`, not a bare npx invocation.
  **Repo**: initialized (`main`) and published public at https://github.com/valenciajoel/kit.
  Work-unit commits are now in effect; `AGENTS.md` + `.gitignore` added with a hard
  "never expose secrets" rule and a pre-commit secret check.
- 2026-10-05: Slice 2 (T7–T8) implemented inline. `go build`, `go vet`, `go test ./...`
  pass. `kit export` produced a 103-file, 230 KB zip and redacted a real
  `CONTEXT7_API_KEY` to `${REDACTED}`; `kit install` emits a 9-run/1-skip plan.
  **Security fix**: the first suffix heuristic (`_key`/`-key`) false-matched
  `topic_key` and the npm package `path-key`, corrupting `package-lock.json` by
  replacing whole objects. Narrowed to specific substrings + precise suffixes; added
  regression tests. **Privacy fix**: bundle metadata no longer records hostname or
  absolute paths — sources are rewritten to `~/...`.
- 2026-10-05: Slice 3 (T9, T11, T10) implemented inline. `go build`, `go vet`,
  `go test ./...` pass (bundle, install, portable, restore). Verified end to end:
  `kit export` rewrote the alacritty Omakub import to
  `~/.config/alacritty/omakub-theme.toml`, bundled the theme (797 B), and
  `kit restore --dry-run` resolved both files to `~/.config/alacritty/*`.
  **Note**: an `accent` test on export/restore between two different home
  directories confirms `~` normalization + per-machine rendering; a Windows-target
  dry-run confirms `%APPDATA%` mapping.
