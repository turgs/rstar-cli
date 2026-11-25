.PHONY: build test vet smoke

build:
	go build -o rstar ./cmd/rstar

test:
	go test ./...

vet:
	go vet ./...

smoke: build
	./rstar version
	./rstar doctor
