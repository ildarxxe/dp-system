package domain

import (
	"context"
	"github.com/minio/minio-go/v7"
	"io"
)

//go:generate mockery --name FileStorage
type FileStorage interface {
	Upload(ctx context.Context, fileName string, content io.Reader, size int64, isResult bool) (string, error)
	UploadByChunks(ctx context.Context, fileName string, content io.Reader, size int64, uploadID string, partID int) (string, error)
	UploadInit(ctx context.Context, fileName string) (string, error)
	UploadFinish(ctx context.Context, fileName, uploadID string, parts []minio.CompletePart) (string, error)
	GeneratePresignedURL(ctx context.Context, bucketName, objectName string) (string, error)
}
