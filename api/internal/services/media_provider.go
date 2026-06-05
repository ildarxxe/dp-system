package services

import (
	"context"
	"dpsystem/domain"
	"dpsystem/internal/constants"
	"dpsystem/pkg/customerror"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
	"io"
	"net/http"
)

type MediaProviderService struct {
	logger    *zap.SugaredLogger
	storage   domain.FileStorage
	mediaRepo domain.MediaProviderRepository
	publisher domain.Publisher
}

func NewMediaProviderService(
	logger *zap.SugaredLogger,
	storage domain.FileStorage,
	mediaRepo domain.MediaProviderRepository,
	publisher domain.Publisher,
) *MediaProviderService {
	return &MediaProviderService{
		logger:    logger,
		storage:   storage,
		mediaRepo: mediaRepo,
		publisher: publisher,
	}
}

func (s *MediaProviderService) ChangeTaskStatus(ctx context.Context, taskID uint64, status string) error {
	err := s.mediaRepo.UpdateStatusAndResultPath(ctx, taskID, status, "")
	if err != nil {
		return err
	}
	return nil
}

func (s *MediaProviderService) ChangeResultPath(ctx context.Context, taskID uint64, path string) error {
	err := s.mediaRepo.UpdateStatusAndResultPath(ctx, taskID, string(constants.StatusSuccess), path)
	if err != nil {
		return err
	}
	return nil
}

func (s *MediaProviderService) UploadInit(ctx context.Context, fileName string) (uint, string, error) {
	uploadID, err := s.storage.UploadInit(ctx, fileName)
	if err != nil {
		return 0, "", err
	}

	task := &domain.MediaTask{
		StoragePath: "",
		ResultPath:  nil,
		FileName:    fileName,
		Size:        0,
		Status:      constants.StatusPending,
	}
	taskID, createErr := s.mediaRepo.CreateTask(ctx, task)
	if createErr != nil {
		s.logger.Errorw("error creating row in database: table MediaTasks", "error", createErr)
		return 0, "", createErr
	}

	return taskID, uploadID, nil
}

func (s *MediaProviderService) UploadByChunks(ctx context.Context, fileName string, content io.Reader, size int64, uploadID string, partID int) (string, error) {
	ETag, err := s.storage.UploadByChunks(ctx, fileName, content, size, uploadID, partID)
	if err != nil {
		return "", err
	}
	return ETag, nil
}

func (s *MediaProviderService) UploadFinish(ctx context.Context, fileName string, size int64, action constants.VideoAction, uploadID string, parts []minio.CompletePart, taskID uint64) error {
	path, err := s.storage.UploadFinish(ctx, fileName, uploadID, parts)
	if err != nil {
		s.logger.Errorw("error uploading file", "error", err, "fileName", fileName, "size", size)
		return customerror.NewCustomError("Ошибка сохранения файла", http.StatusInternalServerError, err)
	}

	task, fetchErr := s.mediaRepo.GetTaskByID(ctx, taskID)
	if fetchErr != nil {
		s.logger.Errorw("error fetching task", "error", fetchErr)
		return fetchErr
	}

	task.Size = size
	task.StoragePath = path
	task.Status = constants.StatusUploaded

	saveErr := s.mediaRepo.SaveTask(ctx, task)
	if saveErr != nil {
		s.logger.Errorw("error saving task", "error", saveErr)
		return saveErr
	}

	publishErr := s.publisher.PublishTask(ctx, uint(taskID), path, action)
	if publishErr != nil {
		s.logger.Errorw("fail to publish task", "taskID", taskID, "error", publishErr)
		return publishErr
	}

	return nil
}
