# forecastui

A terminal dashboard for the Finnish Meteorological Institute's forecast. It
reads FMI's open data (`pal_skandinavia`). Requires a nerd font to be set as the
terminal font for all features.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/olli-io/forecastui/main/install.sh | bash
```

Builds from source into `~/.local/bin` (override with `BINDIR` or `PREFIX`);
needs `git` and a Go toolchain. It is a bash script, so Linux, macOS, WSL and
Git Bash — not PowerShell or `cmd`. The same thing without the pipe:

```bash
go install github.com/olli-io/forecastui/cmd/forecastui@latest
```

That needs only a Go toolchain and works everywhere, dropping the binary in
`$(go env GOPATH)/bin`. Or from a clone:

```bash
go build -o forecastui ./cmd/forecastui
```

## Use

![forecastui](docs/screenshot.png)

