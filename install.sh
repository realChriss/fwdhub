#!/bin/sh
set -eu

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	bold='\033[1m' dim='\033[2m' red='\033[31m' green='\033[32m' yellow='\033[33m' cyan='\033[36m' reset='\033[0m'
else
	bold='' dim='' red='' green='' yellow='' cyan='' reset=''
fi
ok() { printf "  ${green}✓${reset} %b\n" "$1"; }
warn() { printf "  ${yellow}!${reset} %b\n" "$1"; }
fail() {
	printf "  ${red}✗${reset} %b\n\n" "$1" >&2
	exit 1
}
ask() {
	printf "  ${cyan}?${reset} %b ${dim}[Y/n]${reset} " "$1"
	read -r answer </dev/tty || answer=n
	case $answer in [nN]*) return 1 ;; esac
}

printf "\n  ${bold}fwdhub${reset} installer\n\n"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $os in
linux | darwin) ;;
*) fail "Unsupported OS: $os" ;;
esac
case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) fail "Unsupported CPU: $(uname -m)" ;;
esac

dir=$HOME/.local/bin
mkdir -p "$dir"
tmp=$dir/fwdhub.new
trap 'rm -f "$tmp"' EXIT
url=https://github.com/realChriss/fwdhub/releases/latest/download/fwdhub-$os-$arch
if command -v curl >/dev/null; then
	curl -fsSL "$url" -o "$tmp" || fail "Download failed: $url"
elif command -v wget >/dev/null; then
	wget -qO "$tmp" "$url" || fail "Download failed: $url"
else
	fail "Needs curl or wget."
fi
ok "Downloaded fwdhub-$os-$arch ${dim}($(du -h "$tmp" | cut -f1))${reset}"
chmod 755 "$tmp"
mv "$tmp" "$dir/fwdhub"
ok "Installed to ~/.local/bin/fwdhub"

case ":$PATH:" in
*":$dir:"*)
	printf "\n  Run ${bold}fwdhub${reset} to start.\n\n"
	exit 0
	;;
esac

line='export PATH="$HOME/.local/bin:$PATH"'
case ${SHELL##*/} in
zsh) rc=$HOME/.zshrc ;;
bash) rc=$HOME/.bashrc ;;
fish) rc=$HOME/.config/fish/config.fish line='fish_add_path $HOME/.local/bin' ;;
*) rc='' ;;
esac

echo
warn "~/.local/bin is not on your PATH."
if [ -n "$rc" ] && (: </dev/tty) 2>/dev/null && ask "Add it to ~/${rc#"$HOME"/} and restart your shell?"; then
	if ! grep -qsF "$line" "$rc"; then
		mkdir -p "${rc%/*}"
		printf '\n%s\n' "$line" >>"$rc"
	fi
	ok "Added to ~/${rc#"$HOME"/}"
	printf "\n  Run ${bold}fwdhub${reset} to start.\n\n"
	exec "$SHELL" </dev/tty
fi
printf "\n  Add this line to your shell profile, then open a new terminal:\n\n    ${cyan}%s${reset}\n\n" "$line"
