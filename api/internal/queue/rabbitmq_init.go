package queue

import (
	"dpsystem/configs"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"strconv"
	"time"
)

func RabbitMQInit(cfg *configs.RabbitConfig, logger *zap.SugaredLogger) *amqp.Connection {
	var conn *amqp.Connection
	var err error

	maxRetries, _ := strconv.ParseInt(cfg.MaxRetries, 10, 64)
	retryInterval, _ := strconv.ParseInt(cfg.RetryInterval, 10, 64)

	for i := 0; i < int(maxRetries); i++ {
		logger.Info("Attempting to connect to RabbitMQ. Attempt ", i+1)
		conn, err = amqp.Dial(cfg.AmqpUrl)
		if err == nil {
			InitQueues(conn, logger)
			return conn
		}
		logger.Errorw("Failed to connect to RabbitMQ", "error", err)
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}

	return nil
}

func InitQueues(conn *amqp.Connection, logger *zap.SugaredLogger) {
	ch, err := conn.Channel()
	if err != nil {
		logger.Errorw("Failed to open a channel", "error", err)
	}
	defer ch.Close()

	ch.ExchangeDeclare("main_exchange", "direct", true, false, false, false, nil)
	ch.ExchangeDeclare("retry_exchange", "direct", true, false, false, false, nil)

	q, qErr := ch.QueueDeclare("media_tasks", true, false, false, false, amqp.Table{
		//"x-queue-type":           amqp.QueueTypeQuorum, // минимум 3 узла
		"x-dead-letter-exchange": "retry_exchange",
	})
	if qErr != nil {
		logger.Errorw("Failed to declare a queue", "error", qErr)
	}

	ch.QueueBind(q.Name, "task_key", "main_exchange", false, nil)

	rq, retryQErr := ch.QueueDeclare("retry_tasks", true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "main_exchange",
		"x-dead-letter-routing-key": "task_key",
		"x-message-ttl":             int32(5000),
	})
	if retryQErr != nil {
		logger.Errorw("Failed to declare a retry queue", "error", retryQErr)
	}

	ch.QueueBind(rq.Name, "retry_key", "retry_exchange", false, nil)
}
