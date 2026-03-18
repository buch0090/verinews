FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o verinews ./cmd/verinews

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/verinews /usr/local/bin/
CMD ["verinews", "web"]
