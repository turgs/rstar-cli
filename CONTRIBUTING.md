# Contributing

Thanks for helping with the ReceptionStar CLI.

## Setup

1. Install [Go](https://go.dev/dl/) 1.25+.
2. Clone this repository.
3. Run the checks you will see in CI:

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build -o rstar ./cmd/rstar
```

## Pull requests

- Open a PR against `main`.
- Keep the change focused.
- CI must pass before merge.
