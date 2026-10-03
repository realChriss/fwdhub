#!/bin/sh
set -eu

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $os in
linux | darwin) ;;
*) echo "fwdhub: unsupported OS: $os" >&2; exit 1 ;;
esac
case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) echo "fwdhub: unsupported CPU: $(uname -m)" >&2; exit 1 ;;
esac

dir=$HOME/.local/bin
mkdir -p "$dir"
tmp=$dir/fwdhub.new
trap 'rm -f "$tmp"' EXIT
echo "Downloading fwdhub-$os-$arch..."
url=https://github.com/realChriss/fwdhub/releases/latest/download/fwdhub-$os-$arch
if command -v curl >/dev/null; then
	curl -fsSL "$url" -o "$tmp"
else
	wget -qO "$tmp" "$url"
fi
chmod 755 "$tmp"
mv "$tmp" "$dir/fwdhub"
echo "Installed $dir/fwdhub."

case ":$PATH:" in
*":$dir:"*) echo "Run: fwdhub" ;;
*)
	echo "$dir is not on your PATH. Add this line to ~/.zshrc or ~/.bashrc, then open a new terminal:"
	echo '  export PATH="$HOME/.local/bin:$PATH"'
	;;
esac
