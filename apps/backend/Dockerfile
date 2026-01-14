# syntax=docker/dockerfile:1.6

FROM golang:1.22-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api


FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates curl && update-ca-certificates

COPY --from=builder /out/api /app/api

EXPOSE 3001

ENTRYPOINT ["/app/api"]
