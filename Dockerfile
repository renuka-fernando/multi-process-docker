# Stage 1: Build the Go binaries
FROM golang:alpine AS builder

# Install required tools
RUN apk add --no-cache git protobuf-dev

# Install protoc-gen-go and protoc-gen-go-grpc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf code
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/service.proto

# Build server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./server-bin ./server

# Build client
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./client-bin ./client

# Build process manager
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./manager-bin ./manager

# Stage 2: Create minimal runtime image
FROM alpine:3.19

# Create app directory and set proper permissions
RUN mkdir -p /app && \
    chown 10000:10000 /app

# Copy binaries from builder
COPY --from=builder /build/server-bin /app/server
COPY --from=builder /build/client-bin /app/client
COPY --from=builder /build/manager-bin /app/manager

# Make binaries executable
RUN chmod +x /app/server && \
    chmod +x /app/client && \
    chmod +x /app/manager

# Switch to non-root user
USER 10000

# Use our custom process manager as entrypoint
ENTRYPOINT ["/app/manager"]
