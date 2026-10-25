# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o device-service

# Runtime stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata && adduser -D -H app

WORKDIR /app
COPY --from=builder /app/device-service .

USER app
EXPOSE 8082

CMD ["./device-service"]
