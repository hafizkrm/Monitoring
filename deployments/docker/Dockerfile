# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o viscod-agent ./cmd/agent/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/viscod-agent .
COPY config.yaml .
COPY --from=builder /app/backend/internal/database/migrations ./migrations

# Create log directory
RUN mkdir -p ./var/log

# Expose diagnostic port
EXPOSE 8080

# Run the agent
CMD ["./viscod-agent", "-config", "config.yaml"]
