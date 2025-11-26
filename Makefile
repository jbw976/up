# ====================================================================================
# Environment

export CGO_ENABLED=0

# ====================================================================================
# Build Targets

.PHONY: build
build:
	goreleaser build --snapshot --clean --single-target

.PHONY: build.all
build.all:
	goreleaser build --snapshot --clean

.PHONY: generate
generate:
	go generate ./...

# ====================================================================================
# Check Targets

.PHONY: lint
lint: generate
	golangci-lint run --fix

.PHONY: test
test: generate
	go test -v ./...

.PHONY: integration-test
integration-test: generate
	go test -v -tags=integration ./...

.PHONY: reviewable
reviewable: lint test integration-test
	@echo "All checks passed - ready for review"

.PHONY: check-diff
check-diff: generate
	go mod tidy
	git diff --exit-code

# ====================================================================================
# Help

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Build Targets:"
	@echo "  build            Build binaries for current platform"
	@echo "  build.all        Build binaries for all platforms"
	@echo "  generate         Run go generate"
	@echo ""
	@echo "Check Targets:"
	@echo "  check-diff       Generate code and check for uncommitted changes"
	@echo "  lint             Run golangci-lint"
	@echo "  test             Run unit tests"
	@echo "  integration-test Run integration tests"
	@echo "  reviewable       Run all checks (check-diff, lint, test, integration-test)"
	@echo ""
	@echo "Other Targets:"
	@echo "  help             Show this help message"

.DEFAULT_GOAL := help
