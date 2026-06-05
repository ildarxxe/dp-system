package domain

import (
	"context"
	"dpsystem/internal/constants"
	"github.com/minio/minio-go/v7"
	"io"
	"time"
)

type MediaTask struct {
	ID          uint
	StoragePath string
	ResultPath  *string
	FileName    string
	Size        int64
	Status      constants.TaskStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

//go:generate mockery --name MediaProviderRepository
type MediaProviderRepository interface {
	CreateTask(ctx context.Context, task *MediaTask) (uint, error)
	GetTaskByID(ctx context.Context, id uint64) (*MediaTask, error)
	UpdateStatusAndResultPath(ctx context.Context, id uint64, status string, resultPath string) error
	SaveTask(ctx context.Context, task *MediaTask) error
}

//go:generate mockery --name MediaProviderService
type MediaProviderService interface {
	ChangeTaskStatus(ctx context.Context, taskID uint64, status string) error
	ChangeResultPath(ctx context.Context, taskID uint64, path string) error
	UploadInit(ctx context.Context, fileName string) (uint, string, error)
	UploadByChunks(ctx context.Context, fileName string, content io.Reader, size int64, uploadID string, partID int) (string, error)
	UploadFinish(ctx context.Context, fileName string, size int64, action constants.VideoAction, uploadID string, parts []minio.CompletePart, taskID uint64) error
}
