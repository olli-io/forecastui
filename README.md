# forecastui

A terminal dashboard for the Finnish Meteorological Institute's forecast. It
reads FMI's open data (`pal_skandinavia`). Requires a nerd font to be set as the
terminal font for all features.

## Install

Linux and macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/olli-io/forecastui/main/install.sh | bash
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/olli-io/forecastui/main/install.ps1 | iex
```

On a platform with no published binary, `install.sh`
builds from source instead, which needs the Go toolchain and `git`.

[release]: https://github.com/olli-io/forecastui/releases/latest
[releases page]: https://github.com/olli-io/forecastui/releases

### From source

> [!NOTE]
> Requires the Go toolchain and `git`.

```bash
go install github.com/olli-io/forecastui/cmd/forecastui@latest
```

Unlike the install scripts above, this leaves `PATH` alone. Add it to run 'forecastui' in terminal:

```bash
# in ~/.bashrc or ~/.zshrc
export PATH="$PATH:$(go env GOPATH)/bin"
```

```powershell
[Environment]::SetEnvironmentVariable('Path',
  "$([Environment]::GetEnvironmentVariable('Path', 'User'));$(go env GOPATH)\bin",
  'User')
```

## Theming

Colours come from a TOML file in the config directory (`~/.config/forecastui`
on Linux, `~/Library/Application Support/forecastui` on macOS, `%AppData%\forecastui`
on Windows):

```
config.toml          the default place, favourites, and the theme in use
themes/
  everforest.toml    what an install lands on
  default.toml       the terminal's own colours
  catppuccin.toml  dracula.toml  gruvbox-material.toml  kanagawa.toml
  nord.toml  onedarkpro.toml  tokyonight.toml
  <yours>.toml
```

All of them are written out on first run, and an upgrade adds any that are new.
A file already there is left alone, so edits survive. Drop another `.toml`
beside them and name it in `config.toml`:

```toml
theme = "gruvbox-material"
```

`-theme <name>` and `FORECASTUI_THEME` override it for one run, and either
accepts a path to a file outside the themes directory.

Pressing `t` in the dashboard lists them instead. The chart is repainted as the
selection moves, so a theme is judged on the forecast rather than on a swatch;
`enter` keeps the one on screen and writes it to `config.toml`, `esc` puts back
the one that was there.

A theme names the ten slots the chart paints with. Any it leaves out falls back
to the terminal's own colour for that slot, so overriding one takes one line:

```toml
fg     = "#d5c4a1"   # labels
grey   = "#7c6f64"   # axes, grid, headings
dim    = "#504945"   # probability of precipitation, cursor frame, selection
purple = "#d3869b"   # coldest temperatures, moonlit hours
aqua   = "#89b482"   # cold, snow
green  = "#a9b665"
yellow = "#d8a657"   # active tab, keys, clear sky
orange = "#e78a4e"
red    = "#ea6962"   # hottest temperatures, gales, thunder
blue   = "#7daea3"   # rain
```

A colour is a hex value (`"#d5c4a1"`, `"#fff"`), a 0-255 palette index
(`"213"`), one of the sixteen terminal colour names (`"blue"`,
`"bright-black"`), or `"default"` for the terminal's own foreground.
`default.toml` is written in names throughout, so `theme = "default"` follows
whatever the terminal is set to. `NO_COLOR` still turns colour off entirely.

## Screenshot

![forecastui](docs/screenshot.png)

