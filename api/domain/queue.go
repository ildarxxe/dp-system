package domain

import (
	"context"
	"dpsystem/internal/constants"
)

type TaskMessage struct {
	MessageID   string
	TaskID      uint
	StoragePath string
	Action      constants.VideoAction
}

type Publisher interface {
	PublishTask(ctx context.Context, taskID uint, storagePath string, action constants.VideoAction) error
}
