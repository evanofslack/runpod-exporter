FROM golang:1.26 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/runpod-exporter ./cmd/runpod-exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/runpod-exporter /runpod-exporter
EXPOSE 9836
ENTRYPOINT ["/runpod-exporter"]
