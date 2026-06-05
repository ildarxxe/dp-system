FROM mwader/static-ffmpeg:7.1 AS ffmpeg

FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/worker_bin ./cmd/worker/main.go

RUN mkdir -p /app/internal/logs

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=ffmpeg /ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg /ffprobe /usr/local/bin/ffprobe

WORKDIR /app

COPY --from=builder /app/worker_bin ./worker_bin

COPY --from=builder /app/configs ./configs

COPY --from=builder --chown=nonroot:nonroot /app/internal ./internal

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["./worker_bin"]