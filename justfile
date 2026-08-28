set dotenv-load := true

build:
    go build -o bin/runpod-exporter ./cmd/runpod-exporter

run:
    go run ./cmd/runpod-exporter

up:
    docker compose up

down:
    docker compose down

dev-up:
    docker compose -f docker-compose-dev.yaml up --build

dev-down:
    docker compose -f docker-compose-dev.yaml down

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
