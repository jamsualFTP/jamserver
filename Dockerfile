FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o jamsualftp ./cmd

FROM alpine:latest

COPY --from=builder /app/jamsualftp /usr/local/bin/jamsualftp

ENTRYPOINT ["/usr/local/bin/jamsualftp"]
