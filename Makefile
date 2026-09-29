.PHONY: fmt test vet build container

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/mumabridge

container:
	docker build -t mumabridge:dev .
