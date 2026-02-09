FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go build -o discord-relay ./main.go

FROM alpine:3.19
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/discord-relay ./discord-relay
EXPOSE 8080
ENTRYPOINT ["/app/discord-relay"]
