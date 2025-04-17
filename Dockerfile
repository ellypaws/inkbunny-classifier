FROM golang:1.24.2 AS builder

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
# Copy vendor folder and set up module replacement
COPY cmd/dataset/vendor ./cmd/dataset/vendor
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build both binaries in the builder stage
RUN CGO_ENABLED=0 go build -o /app/bin/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /app/bin/telegram ./cmd/telegram

# Server stage
FROM alpine:latest AS server
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/server /app/server
ENV PORT=8080
EXPOSE 8080
CMD ["/app/server"]

# Telegram stage
FROM alpine:latest AS telegram
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/telegram /app/telegram
CMD ["/app/telegram"] 