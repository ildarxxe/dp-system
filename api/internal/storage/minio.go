package storage

import (
	"context"
	"dpsystem/configs"
	"dpsystem/pkg/customerror"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

type MinioStorage struct {
	client        *minio.Core
	signingClient *minio.Core
	cfg           *configs.StorageConfig
	logger        *zap.SugaredLogger
}

func NewMinioStorage(logger *zap.SugaredLogger, cfg *configs.StorageConfig) *MinioStorage {
	minioClient, err := minio.NewCore(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		logger.Errorw("minio initialize error", "error", err)
		return nil
	}

	signingClient, err := minio.NewCore(cfg.PublicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		logger.Errorw("minio signing client initialize error", "error", err)
		signingClient = minioClient
	}

	return &MinioStorage{
		client:        minioClient,
		signingClient: signingClient,
		cfg:           cfg,
		logger:        logger,
	}
}

func (m *MinioStorage) MakeBuckets(ctx context.Context) error {
	uploadsExists, uploadErr := m.client.BucketExists(ctx, m.cfg.UploadsBucket)
	if uploadErr != nil {
		m.logger.Errorw("minio uploads bucket exists error", "error", uploadErr)
		return uploadErr
	}

	completedExists, completedErr := m.client.BucketExists(ctx, m.cfg.ResultsBucket)
	if completedErr != nil {
		m.logger.Errorw("minio results bucket exists error", "error", completedErr)
		return completedErr
	}

	if !uploadsExists {
		makeErr := m.client.MakeBucket(ctx, m.cfg.UploadsBucket, minio.MakeBucketOptions{})
		if makeErr != nil {
			m.logger.Errorw("minio uploads bucket create error", "error", makeErr)
			return makeErr
		}
	}

	if !completedExists {
		makeErr := m.client.MakeBucket(ctx, m.cfg.ResultsBucket, minio.MakeBucketOptions{})
		if makeErr != nil {
			m.logger.Errorw("minio results bucket create error", "error", makeErr)
			return makeErr
		}
	}

	return nil
}

func (m *MinioStorage) Upload(ctx context.Context, fileName string, content io.Reader, size int64, isResult bool) (string, error) {
	var bucketName string
	if isResult {
		bucketName = m.cfg.ResultsBucket
	} else {
		bucketName = m.cfg.UploadsBucket
	}
	info, err := m.client.PutObject(ctx, bucketName, fileName, content, size, "", "", minio.PutObjectOptions{})
	if err != nil {
		m.logger.Errorw("minio upload error", "error", err)
		return "", err
	}

	return info.Key, nil
}

func (m *MinioStorage) UploadByChunks(ctx context.Context, fileName string, content io.Reader, size int64, uploadID string, partID int) (string, error) {
	part, err := m.client.PutObjectPart(ctx, m.cfg.UploadsBucket, fileName, uploadID, partID, content, size, minio.PutObjectPartOptions{})
	if err != nil {
		m.logger.Errorw("minio upload error", "error", err)
		return "", customerror.NewCustomError("[MINIO]: upload error", http.StatusInternalServerError, err)
	}
	return part.ETag, nil
}

func (m *MinioStorage) UploadInit(ctx context.Context, fileName string) (string, error) {
	uploadID, err := m.client.NewMultipartUpload(ctx, m.cfg.UploadsBucket, fileName, minio.PutObjectOptions{})
	if err != nil {
		m.logger.Errorw("minio upload init error", "error", err)
		return "", customerror.NewCustomError("[MINIO]: upload process init error", http.StatusInternalServerError, err)
	}
	return uploadID, nil
}

func (m *MinioStorage) UploadFinish(ctx context.Context, fileName, uploadID string, parts []minio.CompletePart) (string, error) {
	info, err := m.client.CompleteMultipartUpload(ctx, m.cfg.UploadsBucket, fileName, uploadID, parts, minio.PutObjectOptions{})
	if err != nil {
		m.logger.Errorw("minio upload finish error", "error", err)
		return "", customerror.NewCustomError("[MINIO]: upload finish error", http.StatusInternalServerError, err)
	}
	return info.Key, nil
}

func (m *MinioStorage) GetFile(ctx context.Context, bucketName, objectName string) (string, error) {
	filePath := fmt.Sprintf("/tmp/uploaded_%s.mp4", uuid.New().String())
	err := m.client.FGetObject(ctx, bucketName, objectName, filePath, minio.GetObjectOptions{})
	if err != nil {
		m.logger.Errorw("minio get file error", "error", err)
		return "", err
	}
	return filePath, nil
}

func (m *MinioStorage) GeneratePresignedURL(ctx context.Context, bucketName, objectName string) (string, error) {
	u, err := m.signingClient.PresignedGetObject(ctx, bucketName, objectName, time.Hour*12, url.Values{})
	if err != nil {
		m.logger.Errorw("minio presigned get object error", "error", err)
		return "", customerror.NewCustomError("[MINIO]: get object error", http.StatusInternalServerError, err)
	}
	return u.String(), nil
}
