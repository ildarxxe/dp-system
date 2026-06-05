package handlers

import (
	"bytes"
	"dpsystem/domain/mocks"
	"dpsystem/internal/constants"
	"dpsystem/pkg/customerror"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestUploadInit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop().Sugar()

	t.Run("success", func(t *testing.T) {
		mockService := new(mocks.MediaProviderService)
		handler := NewMediaProviderHandler(logger, mockService)

		router := gin.Default()
		router.POST("/upload/init", handler.UploadInit)

		taskID := uint(1)
		uploadID := "upload_123"
		mockService.On("UploadInit", mock.Anything, "test.mp4").Return(taskID, uploadID, nil)

		body := bytes.NewBufferString("file_name=test.mp4")
		req, _ := http.NewRequest(http.MethodPost, "/upload/init", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, float64(taskID), response["task_id"])
		assert.Equal(t, uploadID, response["upload_id"])

		mockService.AssertExpectations(t)
	})

	t.Run("service_error", func(t *testing.T) {
		mockService := new(mocks.MediaProviderService)
		handler := NewMediaProviderHandler(logger, mockService)

		router := gin.Default()
		router.POST("/upload/init", handler.UploadInit)

		mockService.On("UploadInit", mock.Anything, "test.mp4").Return(uint(0), "", errors.New("service error"))

		body := bytes.NewBufferString("file_name=test.mp4")
		req, _ := http.NewRequest(http.MethodPost, "/upload/init", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

}

func TestUploadByChunks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop().Sugar()

	t.Run("success", func(t *testing.T) {
		mockService := new(mocks.MediaProviderService)
		handler := NewMediaProviderHandler(logger, mockService)

		router := gin.Default()
		router.POST("/upload/chunk", handler.UploadByChunks)

		eTag := "etag_123"
		mockService.On("UploadByChunks", mock.Anything, "test.mp4", mock.Anything, int64(10), "upload_123", 1).
			Return(eTag, nil)

		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		writer.WriteField("file_name", "test.mp4")
		writer.WriteField("file_size", "10")
		writer.WriteField("upload_id", "upload_123")
		writer.WriteField("part_number", "1")
		part, _ := writer.CreateFormFile("file", "test.mp4")
		part.Write([]byte("0123456789"))
		writer.Close()

		req, _ := http.NewRequest(http.MethodPost, "/upload/chunk", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, eTag, response["e_tag"])

		mockService.AssertExpectations(t)
	})
}

func TestUploadFinish(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop().Sugar()

	t.Run("success", func(t *testing.T) {
		mockService := new(mocks.MediaProviderService)
		handler := NewMediaProviderHandler(logger, mockService)

		router := gin.Default()
		router.POST("/upload/finish", handler.UploadFinish)

		parts := []Part{{ETag: "tag1", PartNumber: 1}}
		completeParts := []minio.CompletePart{{ETag: "tag1", PartNumber: 1}}

		mockService.On("UploadFinish", mock.Anything, "test.mp4", int64(1024), constants.ActionToMP4, "upload_123", completeParts, uint64(1)).
			Return(nil)

		reqBody := FinishUploadRequest{
			Parts:    parts,
			UploadID: "upload_123",
			FileName: "test.mp4",
			Action:   "mp4",
			Size:     1024,
			TaskID:   1,
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest(http.MethodPost, "/upload/finish", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("service_error", func(t *testing.T) {
		mockService := new(mocks.MediaProviderService)
		handler := NewMediaProviderHandler(logger, mockService)

		router := gin.Default()
		router.POST("/upload/finish", handler.UploadFinish)

		mockService.On("UploadFinish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(customerror.NewCustomError("error", http.StatusInternalServerError, errors.New("service fail")))

		reqBody := FinishUploadRequest{
			Parts:    []Part{},
			UploadID: "upload_123",
			FileName: "test.mp4",
			Action:   "mp4",
			Size:     1024,
			TaskID:   1,
		}
		jsonBytes, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest(http.MethodPost, "/upload/finish", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})
}
