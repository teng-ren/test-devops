FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mock-llm

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/mock-llm .

EXPOSE 8000

CMD ["./mock-llm"]
