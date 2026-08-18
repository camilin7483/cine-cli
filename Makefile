.PHONY: build test vet lint install clean coverage

BINARY := cine
GO := go
GOFLAGS :=

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/cine

test:
	$(GO) test ./... -count=1 -timeout 30s

vet:
	$(GO) vet ./...

lint:
	golangci-lint run

install:
	$(GO) install ./cmd/cine

clean:
	rm -f $(BINARY)
	rm -f coverage.out coverage.html

coverage:
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html
