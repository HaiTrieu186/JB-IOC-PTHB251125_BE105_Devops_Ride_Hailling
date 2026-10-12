FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/dispatch-service .

FROM alpine:3

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /app/dispatch-service /app/dispatch-service

EXPOSE 8003

CMD ["/app/dispatch-service"]
