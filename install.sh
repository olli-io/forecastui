#!/usr/bin/env bash
# Install forecastui into ~/.local/bin (override with PREFIX or BINDIR, pick a
# release with VERSION). Falls back to a source build, which needs Go and git.
set -euo pipefail

repo=olli-io/forecastui
bindir=${BINDIR:-${PREFIX:-$HOME/.local}/bin}
version=${VERSION:-}

die() { echo "forecastui: $*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# One trap for the whole run: a download falling through to a source build makes
# two temporary directories, and a second trap would forget the first.
tmps=()
cleanup() { [ ${#tmps[@]} -eq 0 ] || rm -rf "${tmps[@]}"; }
trap cleanup EXIT
mktmp() { local d; d=$(mktemp -d) || return 1; tmps+=("$d"); printf '%s\n' "$d"; }

# The archives are named after GOOS and GOARCH. An unknown pair leaves these
# empty and falls back to source.
os= arch=
case $(uname -s) in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  MINGW*|MSYS*|CYGWIN*) os=windows ;;
esac
case $(uname -m) in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
esac

fetch() {
  if have curl; then curl -fsSL "$1" -o "$2"
  elif have wget; then wget -qO "$2" "$1"
  else return 1
  fi
}

# Reads the tag from the /releases/latest redirect: no API token, no JSON.
latest_version() {
  local url
  have curl || return 1
  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/$repo/releases/latest") || return 1
  case ${url##*/} in
    ''|latest) return 1 ;;
    *) printf '%s\n' "${url##*/}" ;;
  esac
}

# Checks an archive against SHA256SUMS; no checksum tool warns rather than stops.
verify() { # archive name sumsfile
  local sum want
  if have sha256sum; then sum=$(sha256sum "$1" | cut -d' ' -f1)
  elif have shasum; then sum=$(shasum -a 256 "$1" | cut -d' ' -f1)
  else echo "note: no sha256 tool found, skipping checksum" >&2; return 0
  fi
  want=$(awk -v n="$2" '$2 == n || $2 == "*" n { print $1 }' "$3")
  [ -n "$want" ] || die "no checksum listed for $2"
  [ "$sum" = "$want" ] || die "checksum mismatch for $2"
}

# cp + chmod rather than install(1), which Git Bash does not ship. The copy
# lands beside the target and is renamed over it: overwriting a running binary
# in place is "Text file busy", a rename is not.
install_bin() { # path name
  mkdir -p "$bindir"
  cp "$1" "$bindir/$2.new"
  chmod 0755 "$bindir/$2.new"
  mv -f "$bindir/$2.new" "$bindir/$2"
  echo "installed $bindir/$2"
}

# Reports failure so the caller can build instead.
from_release() {
  [ -n "$os" ] && [ -n "$arch" ] || return 1
  [ -n "$version" ] || version=$(latest_version) || return 1

  local ext=tar.gz bin=forecastui
  if [ "$os" = windows ]; then
    # Only unzip can open the Windows archive; Git Bash's GNU tar cannot.
    have unzip || return 1
    ext=zip bin=forecastui.exe
  fi

  local name="forecastui_${version#v}_${os}_${arch}.$ext"
  local base="https://github.com/$repo/releases/download/$version"
  local tmp
  tmp=$(mktmp) || return 1

  echo "downloading $name..."
  fetch "$base/$name" "$tmp/$name" || return 1
  if fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS"; then
    verify "$tmp/$name" "$name" "$tmp/SHA256SUMS"
  else
    echo "note: SHA256SUMS not published, skipping checksum" >&2
  fi

  if [ "$ext" = zip ]; then
    unzip -qo "$tmp/$name" -d "$tmp" || return 1
  else
    tar -xzf "$tmp/$name" -C "$tmp" || return 1
  fi
  install_bin "$tmp/$bin" "$bin"
}

# Builds the checkout this script sits in. Piped from curl there is none, so it
# clones into a temp directory and throws it away again.
from_source() {
  local src_dir
  src_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]:-$0}")" && pwd)
  if [ ! -f "$src_dir/go.mod" ]; then
    for tool in git go; do
      have "$tool" || die "$tool is required to build from source"
    done
    local tmp
    tmp=$(mktmp)
    echo "fetching source..."
    git clone --quiet --depth 1 "https://github.com/$repo.git" "$tmp/forecastui"
    src_dir=$tmp/forecastui
  fi
  have go || die "go is required to build from source"

  # Under Git Bash or MSYS the binary needs its .exe suffix to be runnable.
  local bin=forecastui$(go env GOEXE)
  echo "building $bin..."
  (cd "$src_dir" && go build -o "$bin" ./cmd/forecastui)
  install_bin "$src_dir/$bin" "$bin"
}

from_release || {
  echo "no release binary available, building from source..." >&2
  from_source
}

case ":$PATH:" in
  *":$bindir:"*) ;;
  *) echo "note: $bindir is not on your PATH" >&2 ;;
esac
