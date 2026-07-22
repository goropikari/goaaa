.PHONY: fmt lint

fmt:
	dprint fmt
	golangci-lint run --fix

lint: fmt
	golangci-lint run
