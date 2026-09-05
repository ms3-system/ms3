FROM golang:1.26.6-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/auth-service ./cmd/server/main.go


FROM alpine:3.23

WORKDIR /app

RUN addgroup -S AuthServiceGroup && adduser -S User -G AuthServiceGroup

COPY --from=builder /app/auth-service .

RUN chown -R User:AuthServiceGroup /app

USER User

EXPOSE 8082

ENTRYPOINT ["./auth-service"]
