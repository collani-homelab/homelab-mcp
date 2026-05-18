# Build Stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install ca-certificates required for HTTPS
RUN apk add -U --no-cache ca-certificates

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /homelab-mcp ./cmd/server

# Final Stage
FROM scratch

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /homelab-mcp /homelab-mcp

# Expose port if SSE transport is used (SSE typically defaults to 8080 or is passed via env)
EXPOSE 8080

ENTRYPOINT ["/homelab-mcp"]
