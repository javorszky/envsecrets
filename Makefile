.PHONY: build lint test vet fmt govulncheck staticcheck trivy check tidy

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

staticcheck:
	golangci-lint run --enable-only staticcheck ./...

trivy:
	trivy fs --scanners vuln,secret --severity CRITICAL,HIGH .

check: lint govulncheck test

tidy:
	go mod tidy && go mod download && go mod vendor
