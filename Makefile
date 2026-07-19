.PHONY: fmt lint

fmt:
	dprint fmt
	golangci-lint run --fix

lint:
	golangci-lint run
