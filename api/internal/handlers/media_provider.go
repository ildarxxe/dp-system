package handlers

import (
	"dpsystem/domain"
	"dpsystem/internal/constants"
	"dpsystem/pkg/customerror"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
	"mime/multipart"
	"net/http"
)

type MediaProviderHandler struct {
	appLogger    *zap.SugaredLogger
	mediaService domain.MediaProviderService
}

type UploadInitRequest struct {
	FileName string `form:"file_name"`
}

type UploadByChunksRequest struct {
	FileSize   int64                 `form:"file_size"`
	FileName   string                `form:"file_name"`
	File       *multipart.FileHeader `form:"file"`
	UploadID   string                `form:"upload_id"`
	PartNumber int                   `form:"part_number"`
}

type Part struct {
	ETag       string `json:"e_tag"`
	PartNumber int    `json:"part_number"`
}

type FinishUploadRequest struct {
	Parts    []Part `json:"parts"`
	UploadID string `json:"upload_id"`
	FileName string `json:"file_name"`
	Action   string `json:"action"`
	Size     int64  `json:"size"`
	TaskID   uint64 `json:"task_id"`
}

func NewMediaProviderHandler(appLogger *zap.SugaredLogger, mediaService domain.MediaProviderService) *MediaProviderHandler {
	return &MediaProviderHandler{
		appLogger:    appLogger,
		mediaService: mediaService,
	}
}

func (h *MediaProviderHandler) UploadInit(c *gin.Context) {
	var req UploadInitRequest
	if err := c.ShouldBind(&req); err != nil {
		h.appLogger.Errorw("validation error", "error", err)
		c.Error(customerror.NewCustomError("Неверные данные.", http.StatusBadRequest, err))
		return
	}

	taskID, uploadID, err := h.mediaService.UploadInit(c, req.FileName)
	if err != nil {
		h.appLogger.Errorw("upload error", "error", err)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id": uploadID,
		"task_id":   taskID,
	})
}

func (h *MediaProviderHandler) UploadByChunks(c *gin.Context) {
	var req UploadByChunksRequest
	if err := c.ShouldBind(&req); err != nil {
		h.appLogger.Errorw("validation error", "error", err)
		c.Error(customerror.NewCustomError("Неверные данные.", http.StatusBadRequest, err))
		return
	}

	file, err := req.File.Open()
	if err != nil {
		c.Error(customerror.NewCustomError("Ошибка при чтении файла.", http.StatusInternalServerError, err))
		return
	}
	defer file.Close()

	ETag, err := h.mediaService.UploadByChunks(c, req.FileName, file, req.FileSize, req.UploadID, req.PartNumber)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"e_tag": ETag,
	})
}

func (h *MediaProviderHandler) UploadFinish(c *gin.Context) {
	var req FinishUploadRequest
	if err := c.ShouldBind(&req); err != nil {
		h.appLogger.Errorw("validation error", "error", err)
		c.Error(customerror.NewCustomError("Неверные данные.", http.StatusBadRequest, err))
		return
	}

	var completeParts []minio.CompletePart
	for _, part := range req.Parts {
		completeParts = append(completeParts, minio.CompletePart{
			ETag:       part.ETag,
			PartNumber: part.PartNumber,
		})
	}

	var action constants.VideoAction
	switch req.Action {
	case "compress":
		action = constants.ActionCompress
	case "mp4":
		action = constants.ActionToMP4
	}

	err := h.mediaService.UploadFinish(c, req.FileName, req.Size, action, req.UploadID, completeParts, req.TaskID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
	})
}
