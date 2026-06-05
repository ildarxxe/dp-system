package services

import (
	"context"
	"dpsystem/domain"
	"dpsystem/domain/mocks"
	"dpsystem/internal/constants"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestChangeTaskStatus(t *testing.T) {
	mockRepo := new(mocks.MediaProviderRepository)
	logger := zap.NewNop().Sugar()
	service := NewMediaProviderService(logger, nil, mockRepo, nil)

	ctx := context.Background()
	taskID := uint64(1)
	status := constants.StatusUploaded

	t.Run("success", func(t *testing.T) {
		mockRepo.On("UpdateStatusAndResultPath", ctx, taskID, string(status), "").Return(nil).Once()
		err := service.ChangeTaskStatus(ctx, taskID, string(status))
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("database_error", func(t *testing.T) {
		dbErr := errors.New("database error")
		mockRepo.On("UpdateStatusAndResultPath", ctx, taskID, string(status), "").Return(dbErr).Once()
		err := service.ChangeTaskStatus(ctx, taskID, string(status))
		assert.Error(t, err)
		assert.Equal(t, dbErr, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestChangeResultPath(t *testing.T) {
	mockRepo := new(mocks.MediaProviderRepository)
	logger := zap.NewNop().Sugar()
	service := NewMediaProviderService(logger, nil, mockRepo, nil)

	ctx := context.Background()
	taskID := uint64(1)
	path := "some/path"

	t.Run("success", func(t *testing.T) {
		mockRepo.On("UpdateStatusAndResultPath", ctx, taskID, string(constants.StatusSuccess), path).Return(nil).Once()
		err := service.ChangeResultPath(ctx, taskID, path)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("database_error", func(t *testing.T) {
		dbErr := errors.New("database error")
		mockRepo.On("UpdateStatusAndResultPath", ctx, taskID, string(constants.StatusSuccess), path).Return(dbErr).Once()
		err := service.ChangeResultPath(ctx, taskID, path)
		assert.Error(t, err)
		assert.Equal(t, dbErr, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestUploadInit(t *testing.T) {
	mockRepo := new(mocks.MediaProviderRepository)
	mockStorage := new(mocks.FileStorage)
	logger := zap.NewNop().Sugar()

	service := NewMediaProviderService(logger, mockStorage, mockRepo, nil)

	testTable := []struct {
		testName      string
		fileName      string
		storageReturn string
		storageErr    error
		dbReturn      uint
		dbErr         error
		wantErr       error
	}{
		{
			testName:      "success",
			fileName:      "test.jpg",
			storageReturn: "upload_1",
			storageErr:    nil,
			dbReturn:      1,
			dbErr:         nil,
			wantErr:       nil,
		},
		{
			testName:      "storage_error",
			fileName:      "test.jpg",
			storageReturn: "",
			storageErr:    errors.New("storage fail"),
			dbReturn:      0,
			dbErr:         nil,
			wantErr:       errors.New("storage fail"),
		},
		{
			testName:      "database_error",
			fileName:      "test.jpg",
			storageReturn: "upload_1",
			storageErr:    nil,
			dbReturn:      0,
			dbErr:         errors.New("db fail"),
			wantErr:       errors.New("db fail"),
		},
	}

	for _, tc := range testTable {
		t.Run(tc.testName, func(t *testing.T) {
			ctx := context.Background()

			mockStorage.On("UploadInit", ctx, tc.fileName).
				Return(tc.storageReturn, tc.storageErr).Once()

			if tc.storageErr == nil {
				mockRepo.On("CreateTask", ctx, mock.AnythingOfType("*domain.MediaTask")).
					Return(tc.dbReturn, tc.dbErr).Once()
			}
			taskID, uploadID, err := service.UploadInit(ctx, tc.fileName)

			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.dbReturn, taskID)
				assert.Equal(t, tc.storageReturn, uploadID)
			}

			mockStorage.ExpectedCalls = nil
			mockRepo.ExpectedCalls = nil
		})
	}
}

func TestUploadByChunks(t *testing.T) {
	mockStorage := new(mocks.FileStorage)
	logger := zap.NewNop().Sugar()

	service := NewMediaProviderService(logger, mockStorage, nil, nil)

	testTable := []struct {
		testName      string
		fileName      string
		content       io.Reader
		size          int64
		uploadID      string
		partID        int
		storageReturn string
		storageErr    error
		wantErr       error
	}{
		{
			testName:      "success",
			fileName:      "test.jpg",
			storageReturn: "upload_1",
			storageErr:    nil,
			content:       strings.NewReader("0123456789"),
			size:          634,
			uploadID:      "1",
			partID:        1,
			wantErr:       nil,
		},
		{
			testName:      "storage_error",
			fileName:      "test.jpg",
			storageReturn: "",
			storageErr:    errors.New("storage fail"),
			content:       strings.NewReader("0123456789"),
			size:          634,
			uploadID:      "1",
			partID:        1,
			wantErr:       errors.New("storage fail"),
		},
	}

	for _, tc := range testTable {
		t.Run(tc.testName, func(t *testing.T) {
			ctx := context.Background()

			mockStorage.On("UploadByChunks", ctx, tc.fileName, tc.content, tc.size, tc.uploadID, tc.partID).Return(tc.storageReturn, tc.storageErr).Once().
				Return(tc.storageReturn, tc.storageErr).Once()

			eTag, err := service.UploadByChunks(ctx, tc.fileName, tc.content, tc.size, tc.uploadID, tc.partID)

			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.storageReturn, eTag)
			}

			mockStorage.ExpectedCalls = nil
		})
	}
}

type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) PublishTask(ctx context.Context, taskID uint, storagePath string, action constants.VideoAction) error {
	args := m.Called(ctx, taskID, storagePath, action)
	return args.Error(0)
}

func TestUploadFinish(t *testing.T) {
	mockRepo := new(mocks.MediaProviderRepository)
	mockStorage := new(mocks.FileStorage)
	mockPublisher := new(MockPublisher)
	logger := zap.NewNop().Sugar()

	service := NewMediaProviderService(logger, mockStorage, mockRepo, mockPublisher)

	testTable := []struct {
		testName       string
		fileName       string
		size           int64
		action         constants.VideoAction
		uploadID       string
		parts          []minio.CompletePart
		taskID         uint64
		storagePath    string
		storageErr     error
		getTaskReturn  *domain.MediaTask
		getTaskErr     error
		saveTaskErr    error
		publishTaskErr error
		wantErr        error
	}{
		{
			testName:       "success",
			fileName:       "test.mp4",
			size:           1024,
			action:         constants.ActionToMP4,
			uploadID:       "upload_1",
			parts:          []minio.CompletePart{},
			taskID:         1,
			storagePath:    "bucket/test.mp4",
			storageErr:     nil,
			getTaskReturn:  &domain.MediaTask{ID: 1},
			getTaskErr:     nil,
			saveTaskErr:    nil,
			publishTaskErr: nil,
			wantErr:        nil,
		},
		{
			testName:    "storage_error",
			fileName:    "test.mp4",
			size:        1024,
			action:      constants.ActionToMP4,
			uploadID:    "upload_1",
			parts:       []minio.CompletePart{},
			taskID:      1,
			storagePath: "",
			storageErr:  errors.New("storage fail"),
			wantErr:     errors.New("storage fail"),
		},
		{
			testName:      "get_task_error",
			fileName:      "test.mp4",
			size:          1024,
			action:        constants.ActionToMP4,
			uploadID:      "upload_1",
			parts:         []minio.CompletePart{},
			taskID:        1,
			storagePath:   "bucket/test.mp4",
			storageErr:    nil,
			getTaskReturn: nil,
			getTaskErr:    errors.New("get task fail"),
			wantErr:       errors.New("get task fail"),
		},
		{
			testName:      "save_task_error",
			fileName:      "test.mp4",
			size:          1024,
			action:        constants.ActionToMP4,
			uploadID:      "upload_1",
			parts:         []minio.CompletePart{},
			taskID:        1,
			storagePath:   "bucket/test.mp4",
			storageErr:    nil,
			getTaskReturn: &domain.MediaTask{ID: 1},
			getTaskErr:    nil,
			saveTaskErr:   errors.New("save task fail"),
			wantErr:       errors.New("save task fail"),
		},
		{
			testName:       "publish_error",
			fileName:       "test.mp4",
			size:           1024,
			action:         constants.ActionToMP4,
			uploadID:       "upload_1",
			parts:          []minio.CompletePart{},
			taskID:         1,
			storagePath:    "bucket/test.mp4",
			storageErr:     nil,
			getTaskReturn:  &domain.MediaTask{ID: 1},
			getTaskErr:     nil,
			saveTaskErr:    nil,
			publishTaskErr: errors.New("publish fail"),
			wantErr:        errors.New("publish fail"),
		},
	}

	for _, tc := range testTable {
		t.Run(tc.testName, func(t *testing.T) {
			ctx := context.Background()

			mockStorage.On("UploadFinish", ctx, tc.fileName, tc.uploadID, tc.parts).
				Return(tc.storagePath, tc.storageErr).Once()

			if tc.storageErr == nil {
				mockRepo.On("GetTaskByID", ctx, tc.taskID).
					Return(tc.getTaskReturn, tc.getTaskErr).Once()

				if tc.getTaskErr == nil {
					mockRepo.On("SaveTask", ctx, mock.AnythingOfType("*domain.MediaTask")).
						Return(tc.saveTaskErr).Once()

					if tc.saveTaskErr == nil {
						mockPublisher.On("PublishTask", ctx, uint(tc.taskID), tc.storagePath, tc.action).
							Return(tc.publishTaskErr).Once()
					}
				}
			}

			err := service.UploadFinish(ctx, tc.fileName, tc.size, tc.action, tc.uploadID, tc.parts, tc.taskID)

			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}

			mockStorage.ExpectedCalls = nil
			mockRepo.ExpectedCalls = nil
			mockPublisher.ExpectedCalls = nil
		})
	}
}
