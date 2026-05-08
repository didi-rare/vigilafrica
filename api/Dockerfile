ARG GO_IMAGE=golang:1.26-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1
ARG ALPINE_IMAGE=alpine:3.22@sha256:310c62b5e7ca5b08167e4384c68db0fd2905dd9c7493756d356e893909057601

FROM ${GO_IMAGE} AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o vigil-api ./cmd/server/main.go

# Final stage
FROM ${ALPINE_IMAGE}

WORKDIR /app

# Install CA certificates for external API requests (NASA EONET)
RUN apk add --no-cache ca-certificates \
	&& addgroup -S app \
	&& adduser -S -G app app

# Copy the binary from builder
COPY --from=builder --chown=app:app /app/vigil-api .

USER app:app

# Expose API port
EXPOSE 8080

# Run the binary
CMD ["./vigil-api"]
