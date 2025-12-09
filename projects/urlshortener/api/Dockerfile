FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /urlshortener .

FROM alpine:latest

COPY --from=builder /urlshortener /urlshortener

EXPOSE 8080

ENTRYPOINT ["/urlshortener"]
