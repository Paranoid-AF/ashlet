BINARY := ashletd
GOFLAGS := -v

.PHONY: build test lint format clean

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/ashletd

test:
	go test ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi

format:
	gofmt -w .

clean:
	rm -f $(BINARY)
