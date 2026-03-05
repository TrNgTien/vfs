# Build stage: CGO_ENABLED=1 required for tree-sitter C bindings
FROM golang:1.24-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /vfs ./cmd/vfs

# Runtime stage
FROM debian:bookworm-slim

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /vfs /usr/local/bin/vfs

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /workspace

EXPOSE 8080 3000

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
