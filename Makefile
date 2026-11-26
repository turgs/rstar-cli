.PHONY: build test vet fmt-check smoke

# Only stamp a release version when HEAD is exactly on a v* tag; otherwise keep 0.0.0-dev
# so `rstar upgrade` treats local builds as non-release.
VERSION ?= $(shell v=$$(git describe --tags --exact-match 2>/dev/null); if [ -n "$$v" ]; then echo "$${v#v}"; else echo 0.0.0-dev; fi)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo)
LDFLAGS  = -s -w -X github.com/turgs/rstar-cli/internal/version.Version=$(VERSION) -X github.com/turgs/rstar-cli/internal/version.Commit=$(COMMIT)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o rstar ./cmd/rstar

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi

smoke: build
	./rstar version
	./rstar doctor >/dev/null || true
	./rstar help >/dev/null
	./rstar help --agent >/dev/null
	./rstar commands --json >/dev/null
	./rstar skill show >/dev/null
	./rstar upgrade --help >/dev/null
