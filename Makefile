GOPACKAGES=$(shell glide novendor)
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: all
all: deps fmt vet test

.PHONY: deps
deps:
	glide install
	go get github.com/mattn/goveralls

.PHONY: fmt
fmt:
	@if [ -n "$$(gofmt -l ${GOFILES})" ]; then echo 'The following files have errors. Please run gofmt -l -w on your code.' && gofmt -l ${GOFILES} && exit 1; fi

.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out ${GOPACKAGES}

.PHONY: vet
vet:
	go vet ${GOPACKAGES}
