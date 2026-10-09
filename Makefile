.PHONY: lint-install lint test


lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.5


lint:
	golangci-lint run --max-issues-per-linter=0 --max-same-issues=0 ./...


# Tests that need PostgreSQL (taskmill) skip unless TEST_POSTGRES_DSN names a database.
test:
	go test ./...
