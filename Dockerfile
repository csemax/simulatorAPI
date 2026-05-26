FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

FROM alpine:3.20

RUN adduser -D -H appuser

WORKDIR /app

COPY --from=builder /app/server /app/server

USER appuser

EXPOSE 8090

ENTRYPOINT ["/app/server"]