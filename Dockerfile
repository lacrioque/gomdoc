# Build stage
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION}" -o gomdoc .

# Final stage
FROM debian:bookworm-slim

# Install ca-certificates and remove apt cache to keep image small
RUN apt-get update && apt-get install -y --no-install-recommends 
    ca-certificates 
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/gomdoc .

# Create a default directory for serving markdown files
RUN mkdir /docs

# Expose the default port
EXPOSE 7331

# Run the binary
ENTRYPOINT ["/app/gomdoc"]
CMD ["-dir", "/docs"]
