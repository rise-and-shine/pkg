.PHONY: lint_install lint


lint_install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.5
	mkdir -p ./bin
	cp "$(GOPATH)/bin/golangci-lint" ./bin/


lint:
	./bin/golangci-lint run --max-issues-per-linter=0 --max-same-issues=0 ./...
