.PHONY: build test coverage-report coverage-check

build:
	go build -o lsh .

test:
	go test ./...

# SDK coverage: which latitudesh-go-sdk methods have a command. See
# CONTRIBUTING.md, "SDK coverage".
coverage-report:
	go run ./cmd/sdkcoverage report -format text

coverage-check:
	go run ./cmd/sdkcoverage check
