.PHONY: build test cover lint tidy check docs docs-check seam-check

.DEFAULT_GOAL := build

build:
	go build ./...

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run

tidy:
	go mod tidy

check: test lint seam-check

docs: ## regenerate per-package READMEs
	go run ./cmd/docsgen

docs-check: docs ## fail if generated READMEs drift
	git diff --exit-code -- '**/README.md' ':(exclude)README.md'

# seam-check enforces shesha's three import seams:
#   - go-sdk (modelcontextprotocol/go-sdk) may be imported only under mcpx/,
#     the sole seam over the MCP protocol library (see mcpx/README.md).
#   - cobra (spf13/cobra) may be imported only under cli/,
#     host/claudecode/hooks/, host/claudecode/statusline/ (if present),
#     host/claudecode/provider*.go, and examples/ — cobra stays out of the
#     cobra-free core and out of host/claudecode/adapter.go (a
#     transport-only stub). examples/ is sample consumer code that
#     demonstrates the cli command-assembly API (cli.ServerCmd,
#     cli.MountProviders) and models downstream consumers, which import
#     cobra by design — it is not core.
#   - termenv (muesli/termenv) may be imported only under style/, the sole
#     seam over the color-rendering library (see style/README.md); every
#     other package (e.g. host/claudecode/statusline/) renders through
#     style.Renderer instead (MC-56).
SEAM_GREP := grep -rl --exclude-dir=.git --exclude-dir=.worktrees --exclude-dir=vendor --exclude-dir=node_modules --include='*.go'

seam-check:
	@bad=$$($(SEAM_GREP) '"github.com/modelcontextprotocol/go-sdk' . | grep -v '^\./mcpx/'); \
	if [ -n "$$bad" ]; then \
		echo "go-sdk imported outside mcpx/:"; echo "$$bad"; exit 1; \
	fi
	@bad=$$($(SEAM_GREP) '"github.com/spf13/cobra"' . \
		| grep -v '^\./cli/' \
		| grep -v '^\./host/claudecode/hooks/' \
		| grep -v '^\./host/claudecode/statusline/' \
		| grep -v '^\./host/claudecode/provider' \
		| grep -v '^\./examples/'); \
	if [ -n "$$bad" ]; then \
		echo "cobra imported outside the allowed seam (cli/, host/claudecode/hooks/, host/claudecode/statusline/, host/claudecode/provider*.go, examples/):"; \
		echo "$$bad"; exit 1; \
	fi
	@bad=$$($(SEAM_GREP) '"github.com/muesli/termenv"' . | grep -v '^\./style/'); \
	if [ -n "$$bad" ]; then \
		echo "termenv imported outside style/:"; echo "$$bad"; exit 1; \
	fi
