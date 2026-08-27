set dotenv-load := true

build:
    go build -o bin/runpod-exporter ./cmd/runpod-exporter

run:
    go run ./cmd/runpod-exporter

dev-up:
    docker compose up --build

dev-down:
    docker compose down

test:
    go test ./...

vet:
    go vet ./...

fmt:
    gofmt -l -w .

generate:
    go generate ./openapi

tidy:
    go mod tidy
