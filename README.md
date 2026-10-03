# fwdhub

A small terminal UI to manage SSH port tunnels.

![fwdhub demo](assets/demo.gif)

## Install

Linux and macOS:

```
curl -fsSL https://raw.githubusercontent.com/realChriss/fwdhub/main/install.sh | sh
```

Windows (PowerShell):

```
irm https://raw.githubusercontent.com/realChriss/fwdhub/main/install.ps1 | iex
```

Or download the binary for your platform from the [latest release](https://github.com/realChriss/fwdhub/releases/latest).

### Build from source

Needs Go.

```
git clone https://github.com/realChriss/fwdhub.git
cd fwdhub
go build -o fwdhub ./src
```

On Windows use `-o fwdhub.exe`.

## Requirements

- The OpenSSH client (`ssh`) in your `PATH`.
- Key based login: a key in your agent or without a passphrase. fwdhub cannot ask for a password.

## Usage

Run `fwdhub` and press `a` to add a tunnel. Press `?` for all keys.

- Local, remote and dynamic (SOCKS) forwards.
- Hosts from `~/.ssh/config` are offered in the form; any other host can be typed.
- Tunnels keep running after fwdhub closes and show up again when it opens.

Tunnels are saved in `tunnels.json` in `~/.config/fwdhub/` (Linux), `~/Library/Application Support/fwdhub/` (macOS) or `%AppData%\fwdhub\` (Windows).

## License

Apache License 2.0. See [LICENSE](LICENSE).
