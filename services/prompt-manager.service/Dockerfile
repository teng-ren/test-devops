FROM golang:1.25.5-alpine AS builder

WORKDIR /app

# Install git for go mod download (some dependencies may need it)
RUN apk add --no-cache git

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o prompt-manager

FROM alpine:3.20

WORKDIR /app

# Install ca-certificates for HTTPS connections
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/prompt-manager .

EXPOSE 8002

CMD ["./prompt-manager"]
