# Build stage: CGO_ENABLED=1 required for tree-sitter C bindings
FROM golang:1.24-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /vfs ./cmd/vfs

# Runtime stage: ca-certificates from builder for HTTPS if ever needed
FROM debian:bookworm-slim

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /vfs /usr/local/bin/vfs

WORKDIR /workspace

EXPOSE 8080 3000

ENTRYPOINT ["vfs"]
CMD ["mcp", "--http", ":8080"]
