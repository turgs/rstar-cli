# rstar

Command-line tool for [ReceptionStar](https://receptionstar.com).

Use ReceptionStar from your terminal — useful on its own, and for coding agents that run shell commands.

## Status

Early preview. The binary currently supports:

- `rstar version` — print the build version
- `rstar doctor` — basic environment check
- `rstar help` — list commands

Authenticated practice commands are on the way. Install and setup details will also appear in the ReceptionStar help centre when they ship.

## Install

### Build from source

You need [Go](https://go.dev/dl/) 1.22 or newer.

```bash
git clone https://github.com/turgs/rstar-cli.git
cd rstar-cli
go build -o rstar ./cmd/rstar
```

Move `rstar` onto your `PATH`, for example:

```bash
mkdir -p ~/.local/bin
mv rstar ~/.local/bin/
```

Release binaries, an install script, and package-manager installs will come later.

## Usage

```bash
rstar version
rstar doctor
rstar help
```

## Configuration

Nothing to configure yet. When sign-in lands, you will connect `rstar` to your practice on [receptionstar.com](https://receptionstar.com).

## Development

```bash
go test ./...
make smoke
```

## License

[MIT](LICENSE)
