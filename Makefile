.PHONY: test test-race lint cover fmt tidy ci

GO ?= go

test:
	$(GO) test -race -count=1 -timeout=60s ./...

test-race: test

lint:
	golangci-lint run ./...

cover:
	$(GO) test -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage.out | tail -1

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

ci: tidy lint test cover
