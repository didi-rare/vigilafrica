FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Tidy the module for Go 1.26
RUN go mod tidy

# Build the binary
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o vigil-api ./cmd/server/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for external API requests (NASA EONET)
RUN apk add --no-cache ca-certificates

# Copy the binary from builder
COPY --from=builder /app/vigil-api .

# Expose API port
EXPOSE 8080

# Run the binary
CMD ["./vigil-api"]
