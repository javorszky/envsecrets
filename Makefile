.PHONY: build lint test vet fmt govulncheck check

build:
	go build -o envsecrets .

lint:
	golangci-lint run ./...

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

govulncheck:
	govulncheck ./...

check: lint govulncheck test
