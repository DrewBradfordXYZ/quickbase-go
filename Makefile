# QuickBase Go SDK Makefile

.PHONY: all generate spec spec-check build test clean

# The oapi-codegen version that produced generated/quickbase.gen.go. Another
# version rewrites unrelated code. It needs Go 1.25 to run (go run fetches
# it); the generated code needs only the runtime pinned in go.mod (v1.8.0,
# Go 1.24).
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.2

# Default target
all: generate build test

# Regenerate code from OpenAPI spec
generate: spec
	@echo "Generating Go client from OpenAPI spec..."
	go run ./cmd/gospec spec/output/quickbase-patched.json .build/openapi-go.json
	$(OAPI_CODEGEN) -config oapi-codegen.yaml .build/openapi-go.json
	@echo "Generating methods and type aliases..."
	go run ./cmd/generate-methods .build/openapi-go.json
	@echo "Tidying dependencies..."
	go mod tidy
	@echo "Done!"

# Stop if the spec submodule is behind the commit this repo records
spec-check:
	@sh scripts/check-spec.sh

# Run spec tools (convert, patch, validate)
spec: spec-check
	@echo "Building patched OpenAPI spec..."
	cd spec && npm run build --silent 2>/dev/null || npx tsx tools/cli.ts build

# Just patch the spec (faster if source hasn't changed)
spec-patch: spec-check
	@echo "Patching OpenAPI spec..."
	cd spec && npx tsx tools/cli.ts patch

# Build the SDK
build:
	@echo "Building..."
	go build ./...

# Run all tests
test:
	@echo "Running tests..."
	go test ./...

# Run unit tests only
test-unit:
	go test ./core/... ./client/... ./auth/...

# Run integration tests (requires .env with credentials)
test-integration:
	go test ./tests/integration/... -v

# Clean generated files
clean:
	rm -f generated/quickbase.gen.go
	rm -f client/builders_generated.go
	rm -f client/results_generated.go

# Install development tools
tools:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.0

# Help
help:
	@echo "Available targets:"
	@echo "  all              - Generate, build, and test (default)"
	@echo "  generate         - Regenerate code from OpenAPI spec"
	@echo "  spec             - Build patched OpenAPI spec"
	@echo "  spec-patch       - Just patch the spec (faster)"
	@echo "  build            - Build the SDK"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  clean            - Remove generated files"
	@echo "  tools            - Install development tools"
