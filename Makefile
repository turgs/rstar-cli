.PHONY: build test vet fmt-check smoke

build:
	go build -trimpath -o rstar ./cmd/rstar

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

.PHONY: catalog-json
catalog-json:
	go run ./cmd/rstar-catalog-export
