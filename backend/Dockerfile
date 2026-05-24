# Stage 1: Build the Go binary using the standard Go Alpine image
FROM golang:1-alpine AS builder

WORKDIR /app

# Copy dependency definition files first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire backend source code
COPY . .

# Compile the release binary
# -tags release activates the production-specific configurations in mode_release.go
# -ldflags "-s -w" strips debug information and symbols to minimize the final binary size
RUN CGO_ENABLED=0 GOOS=linux go build -tags release -ldflags "-s -w" -o game-server ./cmd/server

# Stage 2: Minimal runtime environment
FROM alpine:latest

WORKDIR /app

# Install standard ca-certificates in case the backend makes outgoing TLS requests
RUN apk --no-cache add ca-certificates

# Copy the compiled executable from the builder stage
COPY --from=builder /app/game-server .

# The backend listens on port 8080 by default (per main.go configuration)
EXPOSE 8080

# Execute the game server binary on container start
ENTRYPOINT ["./game-server"]
