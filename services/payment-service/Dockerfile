FROM golang:1.26.2-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -gcflags='all=-lang=go1.26' -o payment-service ./cmd/app

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/payment-service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8086 9096

CMD ["./payment-service"]