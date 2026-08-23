#!/usr/bin/env bash
# Build forecastui and install it into ~/.local/bin (override with PREFIX or BINDIR).
set -euo pipefail

src_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]:-$0}")" && pwd)
# Piped from curl there is no checkout to build: the script is coming down the
# pipe on its own, so it fetches the source it needs into a temp clone and
# throws it away again afterwards.
if [ ! -f "$src_dir/go.mod" ]; then
  for tool in git go; do
    command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 1; }
  done
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  echo "fetching source..."
  git clone --quiet --depth 1 https://github.com/olli-io/forecastui.git "$tmp/forecastui"
  src_dir=$tmp/forecastui
fi

bindir=${BINDIR:-${PREFIX:-$HOME/.local}/bin}
# Under Git Bash or MSYS the binary needs its .exe suffix to be runnable.
bin=forecastui$(go env GOEXE)

echo "building $bin..."
(cd "$src_dir" && go build -o "$bin" ./cmd/forecastui)

mkdir -p "$bindir"
# cp + chmod rather than install(1), which Git Bash does not ship. The copy
# lands beside the target and is moved over it: replacing a running binary in
# place is what "Text file busy" is, and a rename is not that.
cp "$src_dir/$bin" "$bindir/$bin.new"
chmod 0755 "$bindir/$bin.new"
mv -f "$bindir/$bin.new" "$bindir/$bin"
echo "installed $bindir/$bin"

case ":$PATH:" in
  *":$bindir:"*) ;;
  *) echo "note: $bindir is not on your PATH" >&2 ;;
esac
