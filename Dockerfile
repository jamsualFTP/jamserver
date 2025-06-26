FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

COPY /app/db.json /app/db.json
COPY /app/filesystem.json /app/filesystem.json

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -ldflags="-w -s" -o jamserver ./cmd

FROM alpine:latest
COPY --from=builder /app/jamserver /usr/local/bin/jamserver
COPY --from=builder /app/db.json /app/db.json
COPY --from=builder /app/filesystem.json /app/filesystem.json

EXPOSE 2121
EXPOSE 2222
EXPOSE 50000-60000

ENTRYPOINT ["/usr/local/bin/jamserver"]
