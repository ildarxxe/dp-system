package main

import (
	"context"
	"dpsystem/configs"
	"dpsystem/domain"
	"dpsystem/internal/handlers"
	"dpsystem/internal/middleware"
	"dpsystem/internal/queue"
	"dpsystem/internal/repositories"
	"dpsystem/internal/repositories/ms"
	"dpsystem/internal/services"
	"dpsystem/internal/storage"
	grpc_handlers "dpsystem/internal/transport/grpc"
	ws "dpsystem/internal/websocket"
	"dpsystem/pkg/db"
	media_provider_v1 "dpsystem/pkg/gen/media_provider/v1"
	"dpsystem/pkg/logger"
	"dpsystem/pkg/redis"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandle(hub *ws.Hub, logger *zap.SugaredLogger, c *gin.Context) {
	taskIDStr := c.Query("task_id")
	taskID, _ := strconv.ParseUint(taskIDStr, 10, 64)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("upgrader error", zap.Error(err))
		return
	}
	client := &ws.Client{Hub: hub, Conn: conn, Send: make(chan []byte, 256), TaskID: taskID}
	client.Hub.Register <- client

	go client.WritePump()
	client.ReadPump(logger)
}

func main() {
	r := gin.Default()
	r.MaxMultipartMemory = 16 << 20

	hub := ws.NewHub()
	go hub.Run()

	appLogger := logger.InitLogger()
	cfg := configs.LoadConfig()

	storageClient := storage.NewMinioStorage(appLogger, cfg.Storage)
	makeBucketsErr := storageClient.MakeBuckets(context.Background())
	if makeBucketsErr != nil {
		panic(makeBucketsErr)
	}

	database := db.DatabaseInit(cfg.Database, appLogger)
	sqlDB, err := database.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	rabbitConn := queue.RabbitMQInit(cfg.Rabbit, appLogger)
	txManager := ms.NewTxManager(database)
	publisher := queue.NewPublisher(rabbitConn, appLogger)
	redisClient := redis.RedisInit(cfg.Redis, appLogger)
	pubsub := redisClient.Subscribe(context.Background(), "completed_tasks")
	defer pubsub.Close()

	mediaRepo := repositories.NewMediaProviderRepository(database, txManager, appLogger)
	mediaService := services.NewMediaProviderService(appLogger, storageClient, mediaRepo, publisher)
	mediaGRPCHandler := grpc_handlers.NewMediaProviderHandler(mediaService)
	mediaHandler := handlers.NewMediaProviderHandler(appLogger, mediaService)

	ch := pubsub.Channel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		for msg := range ch {
			var message domain.RedisMessage
			if err := json.Unmarshal([]byte(msg.Payload), &message); err != nil {
				appLogger.Errorw("Error to unmarshal json", "err", err)
			}
			hub.SendMessage(message.TaskID, []byte(message.Message))
		}
	}()

	grpcServer := grpc.NewServer()
	go func() {
		listen, err := net.Listen("tcp", fmt.Sprintf(":%d", 50051))
		if err != nil {
			appLogger.Fatalf("failed to listen grpc: %v", err)
			return
		}

		media_provider_v1.RegisterMediaProviderServiceServer(grpcServer, mediaGRPCHandler)
		appLogger.Infof("starting grpc server on :50051")
		if err := grpcServer.Serve(listen); err != nil {
			appLogger.Fatalf("failed to serve grpc: %v", err)
		}
	}()

	r.Use(middleware.LogsMiddleware(appLogger))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/ws", func(c *gin.Context) {
		wsHandle(hub, appLogger, c)
	})

	apiGroup := r.Group("/api/v1")
	apiGroup.POST("/upload", mediaHandler.UploadInit)
	apiGroup.POST("/upload/continue", mediaHandler.UploadByChunks)
	apiGroup.POST("/upload/finish", mediaHandler.UploadFinish)

	srv := &http.Server{
		Addr:           ":" + os.Getenv("APP_PORT"),
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		if listenErr := srv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			appLogger.Fatalf("failed to serve api: %v", listenErr)
		}
	}()

	<-quit
	appLogger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		appLogger.Fatal("Server forced to shutdown:", err)
	}

	grpcServer.GracefulStop()

	if err := pubsub.Close(); err != nil {
		appLogger.Error("Error closing Redis pubsub:", err)
	}
	if err := sqlDB.Close(); err != nil {
		appLogger.Error("Error closing Database connection:", err)
	}
	if err := rabbitConn.Close(); err != nil {
		appLogger.Error("Error closing RabbitMQ connection:", err)
	}

	appLogger.Info("Server exiting")
}
