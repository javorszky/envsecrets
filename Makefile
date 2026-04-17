.PHONY: build lint test vet fmt govulncheck check tidy

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

tidy:
	go mod tidy && go mod download && go mod vendor
