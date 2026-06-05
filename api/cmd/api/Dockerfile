FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/api_bin ./cmd/api/main.go

RUN mkdir -p /app/internal/logs

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/api_bin ./api_bin
COPY --from=builder /app/configs ./configs
# Copy the internal directory (including logs) with correct ownership
COPY --from=builder --chown=nonroot:nonroot /app/internal ./internal

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["./api_bin"]