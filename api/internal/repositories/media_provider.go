package repositories

import (
	"context"
	"dpsystem/domain"
	"dpsystem/internal/constants"
	"dpsystem/internal/repositories/ms"
	"dpsystem/pkg/customerror"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MediaProviderRepository struct {
	db        *gorm.DB
	txManager *ms.TxManager
	logger    *zap.SugaredLogger
}

func NewMediaProviderRepository(db *gorm.DB, txm *ms.TxManager, logger *zap.SugaredLogger) *MediaProviderRepository {
	return &MediaProviderRepository{
		db:        db,
		txManager: txm,
		logger:    logger,
	}
}

type MediaTaskModel struct {
	gorm.Model
	StoragePath string `gorm:"not null"`
	ResultPath  *string
	FileName    string               `gorm:"not null"`
	Size        int64                `gorm:"not null"`
	Status      constants.TaskStatus `gorm:"not null"`
}

func (r *MediaProviderRepository) CreateTask(ctx context.Context, task *domain.MediaTask) (uint, error) {
	db := r.txManager.GetDB(ctx)
	model := &MediaTaskModel{
		StoragePath: task.StoragePath,
		ResultPath:  task.ResultPath,
		FileName:    task.FileName,
		Size:        task.Size,
		Status:      task.Status,
	}
	err := db.Create(model).Error
	if err != nil {
		return 0, customerror.SqlErrorMap(err)
	}
	return model.ID, nil
}

func (r *MediaProviderRepository) GetTaskByID(ctx context.Context, id uint64) (*domain.MediaTask, error) {
	db := r.txManager.GetDB(ctx)
	var model MediaTaskModel
	err := db.First(&model, id).Error
	if err != nil {
		return nil, customerror.SqlErrorMap(err)
	}
	return &domain.MediaTask{
		ID:          model.ID,
		StoragePath: model.StoragePath,
		ResultPath:  model.ResultPath,
		FileName:    model.FileName,
		Size:        model.Size,
		Status:      model.Status,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func (r *MediaProviderRepository) UpdateStatusAndResultPath(ctx context.Context, id uint64, status string, resultPath string) error {
	db := r.txManager.GetDB(ctx)
	err := db.Model(&MediaTaskModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      status,
		"result_path": resultPath,
	}).Error
	if err != nil {
		return customerror.SqlErrorMap(err)
	}
	return nil
}

func (r *MediaProviderRepository) SaveTask(ctx context.Context, task *domain.MediaTask) error {
	db := r.txManager.GetDB(ctx)
	model := &MediaTaskModel{
		StoragePath: task.StoragePath,
		ResultPath:  task.ResultPath,
		FileName:    task.FileName,
		Size:        task.Size,
		Status:      task.Status,
	}
	err := db.Save(model).Error
	if err != nil {
		return customerror.SqlErrorMap(err)
	}
	return nil
}
