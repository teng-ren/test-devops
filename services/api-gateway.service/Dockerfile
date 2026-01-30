FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api-gateway

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/api-gateway .

EXPOSE 8000

CMD ["./api-gateway"]

