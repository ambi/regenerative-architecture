# Repository command map for humans and AI agents.
#
# Run recipes from the repository root. Recipe names are intent-based and set
# their own working directory, so callers do not need to remember the monorepo
# layout.

set shell := ["zsh", "-cu"]

go_cache := env_var_or_default("GOCACHE", "/tmp/ra-idp-go-cache")
golangci_cache := env_var_or_default("GOLANGCI_LINT_CACHE", "/tmp/ra-idp-go-golangci-cache")

# Show this command map.
default:
    @just --list

# Install local dependencies for all Bun workspaces.
setup: install-tools install-ui

# Install dependencies for RA repository tools.
install-tools:
    cd tools && bun install --frozen-lockfile

# Install dependencies for the ra-idp-go React UI.
install-ui:
    cd ra-idp-go/ui && bun install --frozen-lockfile

# Run the standard repository verification suite.
verify: verify-tools verify-go verify-ui

# Verify the ra-idp-go backend with lint and race-enabled tests.
verify-go: lint-go test-go-race

# Run Go lint for ra-idp-go.
lint-go:
    cd ra-idp-go && GOLANGCI_LINT_CACHE={{golangci_cache}} golangci-lint run ./...

# Run Go tests for ra-idp-go.
test-go:
    cd ra-idp-go && GOCACHE={{go_cache}} go test ./...

# Run race-enabled Go tests for ra-idp-go.
test-go-race:
    cd ra-idp-go && GOCACHE={{go_cache}} go test -race ./...

# Build all Go packages for ra-idp-go.
build-go:
    cd ra-idp-go && GOCACHE={{go_cache}} go build ./...

# Verify the ra-idp-go UI with lint, typecheck, and build.
verify-ui: lint-ui typecheck-ui build-ui

# Run UI lint.
lint-ui:
    cd ra-idp-go/ui && bun run lint

# Run UI typecheck.
typecheck-ui:
    cd ra-idp-go/ui && bun run typecheck

# Build the UI.
build-ui:
    cd ra-idp-go/ui && bun run build

# Run the UI golden-path E2E smoke test.
test-ui-e2e:
    cd ra-idp-go/ui && bun run test:e2e

# Verify RA tools and repository YAML/SCL files.
verify-tools: typecheck-tools lint-tools test-tools yaml-check

# Run lint for RA tools.
lint-tools:
    cd tools && bun run lint

# Run typecheck for RA tools.
typecheck-tools:
    cd tools && bun run typecheck

# Run tests for RA tools.
test-tools:
    cd tools && bun test

# Validate work-item and SCL YAML files.
yaml-check:
    cd tools && bun run yaml-check:all

# Regenerate SCL HTML artifacts.
scl-render:
    cd tools && bun run scl-to-html:ra-idp-go && bun run scl-to-html:self

# Start the ra-idp-go API for local UI development.
dev-api:
    cd ra-idp-go && ADDR=:8081 ISSUER=http://localhost:5173 go run ./cmd/ra-idp-go

# Start the ra-idp-go UI dev server.
dev-ui:
    cd ra-idp-go/ui && bun run dev

# Start the Docker Compose development stack.
dev-compose:
    cd ra-idp-go && docker compose -f deploy/docker/docker-compose.dev.yaml up --build
