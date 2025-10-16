# ===========================
# Stage 1: Build Go binary
# ===========================
FROM golang:1.25-alpine AS builder

# Install git for module fetching
RUN apk add --no-cache git

# Set working directory inside builder
WORKDIR /app

# Copy only go.mod and go.sum first for caching
COPY go.mod go.sum ./

# Download dependencies (cached unless mod files change)
RUN go mod download

# Copy all source code
COPY . ./

# Build the Go binary
# Using CGO_ENABLED=0 ensures static binary for minimal Alpine image
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# ===========================
# Stage 2: Minimal runtime
# ===========================
FROM alpine:latest

# Install CA certificates (needed for HTTPS calls)
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the statically built binary from builder
COPY --from=builder /app/server .

# Expose the port your Go server listens on
EXPOSE 8080

# Command to run the server
CMD ["./server"]
