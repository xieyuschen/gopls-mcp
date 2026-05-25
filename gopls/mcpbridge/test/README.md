# gopls-mcp Tests

This directory contains integration tests and end-to-end (E2E) scenarios for the gopls-mcp server.
For detailed documentation, please visit [https://gopls-mcp](https://gopls-mcp).

## Running Tests

```bash
# Run integration tests (tool-level API verification)
go test -v ./integration/...

# Run end-to-end tests (real user scenarios)
go test -v ./e2e/...
```