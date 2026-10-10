FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/user-service .

FROM alpine:3

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /app/user-service /app/user-service

EXPOSE 8001

CMD ["/app/user-service"]
