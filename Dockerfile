FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o obsidisync .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /build/obsidisync /usr/local/bin/obsidisync

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/obsidisync"]
