# 1 - Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /services
COPY . .
RUN go mod download
RUN go build -o blob main.go

# 2 - Final stage
FROM alpine:latest
WORKDIR /services
COPY --from=builder /services/blob /services/blob

CMD ["/services/blob"]