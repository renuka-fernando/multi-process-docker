# Stage 1: Build the Go binaries
# NOTE: Run 'make proto' to generate protobuf files before building
FROM golang:alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

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
