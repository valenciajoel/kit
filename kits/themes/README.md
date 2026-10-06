# Bundled themes

These theme definitions are **vendored from Omakub** so `kit` can apply them on
any operating system, without requiring Omakub to be installed.

- Source: https://github.com/basecamp/omakub (themes + `default/themed/alacritty.toml.tpl`)
- License: MIT (see the upstream repository)

Each `themes/<slug>/` directory contains:

- `zellij.kdl` — the Zellij theme (a `themes { current { ... } }` block).
- `colors.toml` — the palette, used to render app configs.

`alacritty.toml.tpl` is the shared Alacritty template; `kit` substitutes the
`{{ key }}` placeholders from the chosen theme's `colors.toml` to produce an
Alacritty theme, exactly like Omakub's `omakub-theme-set-templates`.
