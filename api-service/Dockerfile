FROM golang:1.26.6-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api-service ./cmd/server/main.go


FROM alpine:3.23

WORKDIR /app

RUN addgroup -S ApiServiceGroup && adduser -S User -G ApiServiceGroup

COPY --from=builder /app/api-service .

RUN chown -R User:ApiServiceGroup /app

USER User

EXPOSE 8080

ENTRYPOINT ["./api-service"]
