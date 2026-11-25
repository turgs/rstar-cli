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
	./rstar doctor
	./rstar help >/dev/null
