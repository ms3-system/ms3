FROM golang:1.26.6-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/data-service ./cmd/server/main.go


FROM alpine:3.23

WORKDIR /app

RUN addgroup -S DataServiceGroup \
    && adduser -S User -G DataServiceGroup \
    && install -d -o User -g DataServiceGroup /data

COPY --from=builder /app/data-service .

RUN chown -R User:DataServiceGroup /app

USER User

EXPOSE 8081

ENTRYPOINT ["./data-service"]
