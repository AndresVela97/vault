FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o vault ./cmd/server/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/vault .
COPY --from=builder /app/static ./static
COPY --from=builder /app/migrations ./migrations
EXPOSE 4000
CMD ["./vault"]
