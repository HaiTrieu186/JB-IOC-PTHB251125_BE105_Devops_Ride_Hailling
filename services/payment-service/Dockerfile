FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/payment-service .

FROM alpine:3

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /app/payment-service /app/payment-service

EXPOSE 8005

CMD ["/app/payment-service"]
