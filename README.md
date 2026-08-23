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

Both scripts download the binary for your platform from the latest [release],
check it against the published `SHA256SUMS`, and install it — `install.sh` into
`~/.local/bin` (override with `BINDIR` or `PREFIX`), warning you if that is not
on your `PATH`; `install.ps1` into `%LOCALAPPDATA%\Programs\forecastui`
(override with `FORECASTUI_BINDIR`), which it adds to your user `PATH` for you.
Set `VERSION` (or `FORECASTUI_VERSION` on Windows) to a tag such as `v1.0.0` to
install a specific release. On a platform with no published binary, `install.sh`
builds from source instead, which needs the Go toolchain and `git`.

Prebuilt binaries are also on the [releases page] if you would rather unpack one
yourself.

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

## Screenshot

![forecastui](docs/screenshot.png)

## Releasing

Pushing a `v*` tag runs [`.github/workflows/release.yml`](.github/workflows/release.yml),
which tests, cross-compiles for Linux, macOS and Windows on amd64 and arm64, and
publishes the archives and their checksums as a GitHub release:

```bash
git tag -a v1.0.0 -m v1.0.0
git push origin v1.0.0
```
