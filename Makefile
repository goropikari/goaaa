.PHONY: fmt lint gitleaks

fmt:
	dprint fmt
	golangci-lint run --fix

lint: fmt gitleaks
	golangci-lint run

gitleaks:
	gitleaks detect --no-banner --redact --source .
