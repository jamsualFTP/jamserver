FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o jamserver ./cmd

FROM alpine:latest

COPY --from=builder /app/jamserver /usr/local/bin/jamserver

ENTRYPOINT ["/usr/local/bin/jamserver"]
