# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git and certs (needed for secure connections and go module downloads)
RUN apk add --no-cache git ca-certificates

# Copy go.mod and go.sum first to cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o nit-backend cmd/nit/main.go

# Run stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates to verify SSL certificates in outgoing HTTPS calls (e.g. federation)
RUN apk add --no-cache ca-certificates

# Copy the compiled binary from the build stage
COPY --from=builder /app/nit-backend .

# Copy environment example for reference
COPY .env.example .env

# Expose port
EXPOSE 8080

# Run the application
CMD ["./nit-backend"]
