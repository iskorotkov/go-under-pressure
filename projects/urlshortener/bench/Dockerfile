FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bench .

FROM alpine:latest

COPY --from=builder /bench /bench

ENTRYPOINT ["/bench"]
