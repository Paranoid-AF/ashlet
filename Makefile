.PHONY: build test lint format clean

build:
	$(MAKE) -C daemon build
	$(MAKE) -C model build

test:
	$(MAKE) -C daemon test
	@if command -v bats >/dev/null 2>&1; then \
		bats shell/tests/; \
	else \
		echo "bats not found, skipping shell tests"; \
	fi

lint:
	$(MAKE) -C daemon lint
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck --exclude=SC1091 shell/*.sh shell/*.bash; \
	else \
		echo "shellcheck not found, skipping shell lint"; \
	fi

format:
	$(MAKE) -C daemon format
	@if command -v shfmt >/dev/null 2>&1; then \
		shfmt -w -i 2 -bn -ci shell/*.sh shell/*.bash; \
	else \
		echo "shfmt not found, skipping shell format"; \
	fi

clean:
	$(MAKE) -C daemon clean
	$(MAKE) -C model clean
