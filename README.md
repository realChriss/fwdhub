# fwdhub

A small terminal UI to manage SSH port tunnels.

```
   fwdhub   1 up · 1 error · 4 total

    NAME      TYPE      LOCAL                REMOTE             VIA       STATUS
  ───────────────────────────────────────────────────────────────────────────────────
    prod-db   local     localhost:5432   →   db.internal:5432   bastion   ● up 2h 14m
    socks     dynamic   localhost:1080   →   SOCKS proxy        home      ○ stopped
  ▌ webhook   remote    localhost:3000   ←   0.0.0.0:9000       me@vps    ◐ connecting
    grafana   local     localhost:3001   →   grafana:3000       bastion   ✕ port 3001 in use

  enter start/stop  a add  e edit  d delete  l log  X stop all  ? help  q quit
```

## Install

Download the binary for your platform from the [latest release](https://github.com/realChriss/fwdhub/releases/latest).

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
