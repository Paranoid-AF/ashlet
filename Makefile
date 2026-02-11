.PHONY: build repl test lint format clean bootstrap

bootstrap:
	go mod download

build:
	go build -v -o ashletd ./serve

repl:
	go build -v -o ashlet-repl ./repl
	exec ./ashlet-repl

test:
	go test ./...
	@if command -v bats >/dev/null 2>&1; then \
		bats shell/tests/; \
	else \
		echo "bats not found, skipping shell tests"; \
	fi

lint:
	go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not found, skipping"; \
	fi
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck _run.sh shell/_run.sh; \
	else \
		echo "shellcheck not found, skipping shell lint"; \
	fi

format:
	gofmt -w .
	@if command -v shfmt >/dev/null 2>&1; then \
		shfmt -w -i 2 -bn -ci _run.sh shell/_run.sh; \
	else \
		echo "shfmt not found, skipping shell format"; \
	fi

clean:
	rm -f ashletd ashlet-repl
