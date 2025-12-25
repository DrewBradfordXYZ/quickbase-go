# QuickBase Go SDK Makefile

.PHONY: all generate spec build test clean

# Default target
all: generate build test

# Regenerate code from OpenAPI spec
generate: spec
	@echo "Generating Go client from OpenAPI spec..."
	oapi-codegen -config oapi-codegen.yaml spec/output/quickbase-patched.json
	@echo "Generating wrapper methods..."
	cd cmd/generate-wrappers && go run main.go
	@echo "Tidying dependencies..."
	go mod tidy
	@echo "Done!"

# Run spec tools (convert, patch, validate)
spec:
	@echo "Building patched OpenAPI spec..."
	cd spec && npm run build --silent 2>/dev/null || npx tsx tools/cli.ts build

# Just patch the spec (faster if source hasn't changed)
spec-patch:
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
	rm -f client/api_generated.go

# Install development tools
tools:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

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
