package queue

import (
	"context"
	"dpsystem/domain"
	"dpsystem/internal/constants"
	"dpsystem/pkg/customerror"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Publisher struct {
	logger *zap.SugaredLogger
	conn   *amqp.Connection
}

func NewPublisher(conn *amqp.Connection, logger *zap.SugaredLogger) *Publisher {
	return &Publisher{
		logger: logger,
		conn:   conn,
	}
}

func (p *Publisher) PublishTask(ctx context.Context, taskID uint, storagePath string, action constants.VideoAction) error {
	ch, err := p.conn.Channel()
	if err != nil {
		p.logger.Errorw("Failed to open a channel", "error", err)
		return customerror.NewCustomError("Произошла техническая ошибка. Попробуйте позже", http.StatusInternalServerError, err)
	}
	defer ch.Close()

	ch.Confirm(false)

	body := domain.TaskMessage{
		MessageID:   uuid.New().String(),
		TaskID:      taskID,
		StoragePath: storagePath,
		Action:      action,
	}

	bodyJson, _ := json.Marshal(body)

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	publishErr := ch.PublishWithContext(
		ctx,
		"main_exchange",
		"task_key",
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "text/plain",
			Body:         bodyJson,
		},
	)
	if publishErr != nil {
		p.logger.Errorw("Failed to publish a message", "error", publishErr)
		return customerror.NewCustomError("Произошла техническая ошибка. Попробуйте позже", http.StatusInternalServerError, publishErr)
	}

	select {
	case confirmed := <-confirms:
		if confirmed.Ack {
			//
		} else {
			//
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second * 5):
		return fmt.Errorf("timeout waiting for ack")
	}
	return nil
}
