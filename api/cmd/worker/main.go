package main

import (
	"bufio"
	"context"
	"dpsystem/configs"
	"dpsystem/domain"
	"dpsystem/internal/constants"
	"dpsystem/internal/queue"
	"dpsystem/internal/storage"
	media_provider_v1 "dpsystem/pkg/gen/media_provider/v1"
	"dpsystem/pkg/logger"
	"dpsystem/pkg/redis"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func getVideoDuration(path string) (int64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	durationStr := strings.TrimSpace(string(out))
	durationSec, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, err
	}
	return int64(durationSec * 1000), nil
}

func main() {
	const workerCount = 5
	appLogger := logger.InitLogger()
	cfg := configs.LoadConfig()
	redisClient := redis.RedisInit(cfg.Redis, appLogger)
	storageClient := storage.NewMinioStorage(appLogger, cfg.Storage)

	retryPolicy := `{
		"methodConfig": [{
			"name": [{"service": "media_provider_v1.MediaProviderService"}],
			"waitForReady": true,
			"retryPolicy": {
				"MaxAttempts": 5,
				"InitialBackoff": "0.1s",
				"MaxBackoff": "1s",
				"BackoffMultiplier": 2.0,
				"RetryableStatusCodes": ["UNAVAILABLE"]
			}
		}]
	}`

	grpcConn, grpcErr := grpc.NewClient("api:50051", grpc.WithDefaultServiceConfig(retryPolicy), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if grpcErr != nil {
		appLogger.Fatalf("failed to connect to grpc: %v", grpcErr)
		panic(grpcErr)
	}
	defer grpcConn.Close()

	client := media_provider_v1.NewMediaProviderServiceClient(grpcConn)

	conn := queue.RabbitMQInit(cfg.Rabbit, appLogger)

	ch, chErr := conn.Channel()
	if chErr != nil {
		appLogger.Fatalw("failed to connection chanel", "error", chErr)
		panic(chErr)
	}
	defer ch.Close()

	q, qErr := ch.QueueDeclare("media_tasks", true, false, false, false, amqp.Table{

		"x-dead-letter-exchange": "retry_exchange",
	})
	if qErr != nil {
		appLogger.Errorw("Failed to declare a queue", "error", qErr)
	}

	ch.Qos(1, 0, false)

	msgs, listenErr := ch.Consume(q.Name, "", false, false, false, false, nil)
	if listenErr != nil {
		appLogger.Errorw("Failed to register a consumer", "error", listenErr)
		panic(listenErr)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if err := recover(); err != nil {
					appLogger.Errorw("recovered from panic", "error", err, "stack", string(debug.Stack()))
				}
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-msgs:
					if !ok {
						return
					}
					func() {
						var decodedMsg domain.TaskMessage
						json.Unmarshal(msg.Body, &decodedMsg)

						msgExists, _ := redisClient.Exists(ctx, decodedMsg.MessageID).Result()
						if msgExists != 1 {
							filePath, getFileErr := storageClient.GetFile(ctx, cfg.Storage.UploadsBucket, decodedMsg.StoragePath)
							defer os.Remove(filePath)
							if getFileErr != nil {
								appLogger.Errorw("Failed to get file from storage", "error", getFileErr)
								msg.Nack(false, false)
								req := media_provider_v1.ChangeStatusRequest{
									TaskId: uint64(decodedMsg.TaskID),
									Status: string(constants.StatusFailed),
								}
								_, err := client.ChangeTaskStatus(ctx, &req)
								if err != nil {
									appLogger.Errorw("Error to change task status", "err", err)
								}
								return
							}
							outputPath := fmt.Sprintf("/tmp/processed_%s.mp4", uuid.New().String())
							defer os.Remove(outputPath)

							totalDurationMs, durErr := getVideoDuration(filePath)
							if durErr != nil {
								appLogger.Errorw("Failed to get video duration", "error", durErr)
								totalDurationMs = 1
							}
							appLogger.Infof("Total duration for task %d: %d ms", decodedMsg.TaskID, totalDurationMs)

							var args []string
							args = append(args, "-i", filePath)
							switch decodedMsg.Action {
							case constants.ActionCompress:
								args = append(args, "-vcodec", "libx264", "-crf", "28")
							case constants.ActionToMP4:
								args = append(args, "-f", "mp4")
							default:
								appLogger.Errorw("Unsupported action", "action", decodedMsg.Action)
								msg.Nack(false, false)
								req := media_provider_v1.ChangeStatusRequest{
									TaskId: uint64(decodedMsg.TaskID),
									Status: string(constants.StatusFailed),
								}
								_, err := client.ChangeTaskStatus(ctx, &req)
								if err != nil {
									appLogger.Errorw("Error to change task status", "err", err)
								}
								return
							}
							args = append(args, "-progress", "pipe:1", "-nostats")
							args = append(args, outputPath)

							cmd := exec.CommandContext(ctx, "ffmpeg", args...)
							stdout, outErr := cmd.StdoutPipe()
							if outErr != nil {
								appLogger.Errorw("Failed to listen stdout", "error", outErr)
							}
							if err := cmd.Start(); err != nil {
								appLogger.Errorw("Failed to start exec command", "error", err)
								msg.Nack(false, true)
								req := media_provider_v1.ChangeStatusRequest{
									TaskId: uint64(decodedMsg.TaskID),
									Status: string(constants.StatusFailed),
								}
								_, err := client.ChangeTaskStatus(ctx, &req)
								if err != nil {
									appLogger.Errorw("Error to change task status", "err", err)
									return
								}
								return
							}

							scanner := bufio.NewScanner(stdout)
							var lastProgress float64 = -1
							var lastUpdateTime time.Time
							var currentTimeMs int64

							for scanner.Scan() {
								line := scanner.Text()
								if line == "progress=end" {
									break
								}

								if strings.HasPrefix(line, "out_time_ms=") {
									parts := strings.Split(line, "=")
									if len(parts) == 2 {
										rawVal, _ := strconv.ParseInt(parts[1], 10, 64)
										currentTimeMs = rawVal / 1000
									}
								}

								if strings.HasPrefix(line, "progress=") {
									progress := (float64(currentTimeMs) / float64(totalDurationMs)) * 100
									if progress > 100 {
										progress = 100
									}

									if (progress-lastProgress >= 1.0 || time.Since(lastUpdateTime) >= time.Second) && lastProgress < 100 {
										lastProgress = progress
										lastUpdateTime = time.Now()

										appLogger.Infof("Task %d progress: %.2f%%", decodedMsg.TaskID, progress)

										redisMessage := &domain.RedisMessage{
											Message: fmt.Sprintf("Процесс обработки: %.1f%%", progress),
											TaskID:  uint64(decodedMsg.TaskID),
										}

										payload, marshalErr := json.Marshal(redisMessage)
										if marshalErr != nil {
											appLogger.Errorw("Failed to marshal redis message", "error", marshalErr)
										} else {
											redisClient.Publish(ctx, "completed_tasks", payload)
										}
										if progress >= 100 {
											lastProgress = 100
										}
									}
								}
							}
							if err := cmd.Wait(); err != nil {
								appLogger.Errorw("Failed to exec command", "error", err)
								msg.Nack(false, false)
								req := media_provider_v1.ChangeStatusRequest{
									TaskId: uint64(decodedMsg.TaskID),
									Status: string(constants.StatusFailed),
								}
								_, err := client.ChangeTaskStatus(ctx, &req)
								if err != nil {
									appLogger.Errorw("Error to change task status", "err", err)
								}
								return
							}
							file, fileErr := os.Open(outputPath)
							if fileErr != nil {
								appLogger.Errorw("Failed to open file", "error", fileErr)
								return
							}
							defer file.Close()

							fileInfo, _ := file.Stat()
							fileName := filepath.Base(outputPath)

							resultPath, uploadErr := storageClient.Upload(ctx, fileName, file, fileInfo.Size(), true)
							if uploadErr != nil {
								appLogger.Errorw("Failed to upload result video", "error", uploadErr)
							}

							req := media_provider_v1.ChangeResultPathRequest{
								TaskId: uint64(decodedMsg.TaskID),
								Path:   resultPath,
							}
							_, err := client.ChangeTaskResultPath(ctx, &req)
							if err != nil {
								appLogger.Errorw("Error to change task result path", "err", err)
							}

							statusReq := media_provider_v1.ChangeStatusRequest{
								TaskId: uint64(decodedMsg.TaskID),
								Status: string(constants.StatusSuccess),
							}
							_, err = client.ChangeTaskStatus(ctx, &statusReq)
							if err != nil {
								appLogger.Errorw("Error to change task status", "err", err)
							}

							url, urlErr := storageClient.GeneratePresignedURL(ctx, cfg.Storage.ResultsBucket, resultPath)
							if urlErr != nil {
								appLogger.Errorw("Error to generate presigned url", "error", urlErr)
							}

							redisMessage := &domain.RedisMessage{
								Message: fmt.Sprintf("Ваше видео готово, результат доступен в течение 12 часов по этой ссылке: %s", url),
								TaskID:  uint64(decodedMsg.TaskID),
							}

							payload, marshalErr := json.Marshal(redisMessage)
							if marshalErr != nil {
								appLogger.Errorw("Failed to marshal redis message", "error", marshalErr)
							} else {
								redisClient.Publish(ctx, "completed_tasks", payload)
							}

							appLogger.Infow("Task done successfully.", "taskID", decodedMsg.TaskID)
							msg.Ack(false)

							err = redisClient.Set(ctx, decodedMsg.MessageID, "true", 3*time.Hour).Err()
							if err != nil {
								appLogger.Errorw("failed to set msg", "error", err)
							}

							return
						} else {
							msg.Ack(false)
							return
						}
					}()
				}
			}
		}()
	}

	<-quit
	appLogger.Info("Shutting down worker...")
	cancel()

	appLogger.Info("Waiting for workers to finish...")
	wg.Wait()

	appLogger.Info("Closing connections...")

	appLogger.Info("Worker exiting")
}
