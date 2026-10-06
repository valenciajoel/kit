# kit

A cross-platform TUI that detects, exports, and installs a terminal/dev kit —
**Zellij + Alacritty + nvim (LazyVim) + starship + mise + opencode** — on Linux,
WSL2, and (planned) Windows native.

The goal: move a full development environment between machines (including the
two sides of a dual boot) from one portable bundle, without ever leaking a secret.

## Status

Early but functional. `detect`, `export`, `install`, and `restore` work end to
end on Linux; the `wsl` and `windows` targets are modelled and used for path
resolution, with real-machine testing still pending.

## Install

Prebuilt binaries (Linux and macOS):

    curl -fsSL https://valenciajoel.github.io/kit/install.sh | sh

Same script straight from the repository, if you prefer:

    curl -fsSL https://raw.githubusercontent.com/valenciajoel/kit/main/install.sh | sh

Windows: download `kit-windows-amd64.exe` from the
[releases page](https://github.com/valenciajoel/kit/releases).

With a Go toolchain (1.26+):

    go install github.com/valenciajoel/kit/cmd/kit@latest

From source:

    git clone https://github.com/valenciajoel/kit
    cd kit && make build     # -> bin/kit

## Requirements

- Go 1.26 or newer only if you build from source; prebuilt binaries need nothing.

## Build and run

    go build ./...
    go test ./...
    go run ./cmd/kit            # launch the TUI
    go run ./cmd/kit detect     # environment + component status

Or with the Makefile:

    make build                  # -> bin/kit
    make build-linux            # -> bin/kit-linux-amd64
    make build-windows          # -> bin/kit-windows-amd64.exe
    make test

## Usage

    kit            launch the TUI (Dashboard / Inventory / Install / Actions)
    kit detect     print OS, arch, distro, WSL flag, and per-component status
    kit presets    list available component presets
    kit update     check for and install a newer kit release
                     --check         only report whether an update is available
                     -y, --yes       update without asking
                     --force         reinstall even if already current
    kit setup      install tools AND write configs in one run (dry-run by default)
                     --apply         execute; without it this is a dry-run
                     -i, --interactive  guided: accept each component one by one
                     --from FILE     bundle zip to restore configs from (default: seeds)
                     --force         overwrite existing config files
                     --skip-tools    do not install tools
                     --skip-configs  do not write config files
                     --preset NAME   component preset (see kit presets)
                     --components CSV  comma-separated component ids
                     --target T      destination target: linux|wsl|windows
    kit export     capture the kit as a secret-sanitized bundle (zip)
                     --out DIR       output directory (default "bundles")
                     --dry-run       report captured files and redactions, write nothing
                     --preset NAME   component preset (see kit presets)
                     --components CSV  comma-separated component ids
    kit install    print (and optionally run) the tool install plan
                     --apply         execute the plan; without it this is a dry-run
                     -i, --interactive  accept each component one by one
                     --preset NAME   component preset (see kit presets)
                     --components CSV  comma-separated component ids
    kit restore    apply a bundle's configs to this machine
                     --dry-run       report destinations without writing
                     --force         overwrite existing files
                     --target T      destination target: linux|wsl|windows

## How it works

Everything is driven by a declarative manifest, `kits/kit.yaml`, embedded in the
binary. Each component declares how to detect it, which config files it owns,
and how to install it per target:

```yaml
- id: zellij
  name: Zellij
  kind: terminal-multiplexer
  bin: zellij
  configs:
    - src: "~/.config/zellij/config.kdl"
      dest:
        linux: "~/.config/zellij/config.kdl"
        windows: "%APPDATA%/zellij/config.kdl"
  install:
    linux:   { method: apt,    pkg: zellij }
    windows: { method: winget, pkg: "Zellij.Zellij" }
  seed: configs/zellij/config.kdl
```

Install methods: `apt`, `mise`, `npm`, `script`, `winget`, `cargo`, `manual`.

### Selecting components

Nothing has to be all-or-nothing. Choose what to install with a preset or an
explicit list, on `setup`, `install`, and `export`:

    kit presets                          # list available presets
    kit setup --preset minimal           # a named subset
    kit setup --components zellij,starship
    kit export --preset terminal

Presets live in `kits/kit.yaml` (`minimal`, `terminal`, `languages`, `editor`,
`ai`, `full`). An empty selection means everything.

The TUI's **Install** tab does the same interactively: toggle components with
space, cycle presets with `p`, and press enter to install the selection (the
TUI exits and the install runs with a normal terminal so prompts work).

On startup the TUI also checks for a newer release (non-blocking) and shows an
update banner when one is available; run `kit update` to apply it.

### Portability

- `export` rewrites absolute home paths to `~` and repoints third-party imports
  (for example the Omakub alacritty theme) at a bundled copy, so a config keeps
  working on a machine that never had that dependency.
- `restore` renders `~` back to the destination machine's home directory and
  resolves each file against its per-target destination.
- Bundles never record the hostname or absolute source paths.

## Security

This is a **public** repository. See `AGENTS.md` for the full policy.

- `export` sanitizes every captured config: keys matching credential patterns
  (`api_key`, `*_TOKEN`, `password`, `authorization`, `client_secret`, ...) and
  recognizable credential values are replaced with `${REDACTED}`.
- Real credentials belong in environment variables or an untracked local file,
  never in a tracked file.
- Never commit a secret. If one is committed, rotate it immediately.

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

## Roadmap

- Real-machine WSL2 and native Windows validation.
- Windows-native installers.
- More seed configs and richer manifest coverage.
