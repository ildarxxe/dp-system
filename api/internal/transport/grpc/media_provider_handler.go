package grpc

import (
	"context"
	"dpsystem/domain"
	desc "dpsystem/pkg/gen/media_provider/v1"
)

type MediaProviderHandler struct {
	desc.UnimplementedMediaProviderServiceServer
	service domain.MediaProviderService
}

func NewMediaProviderHandler(service domain.MediaProviderService) *MediaProviderHandler {
	return &MediaProviderHandler{
		service: service,
	}
}

func (h *MediaProviderHandler) GetTaskStatus(ctx context.Context, req *desc.GetTaskStatusRequest) (*desc.TaskData, error) {
	//TODO implement me
	panic("implement me")
}

func (h *MediaProviderHandler) ReportProgress(ctx context.Context, req *desc.ReportProgressRequest) (*desc.ProgressResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (h *MediaProviderHandler) GetTaskDetails(ctx context.Context, req *desc.GetTaskDetailsRequest) (*desc.TaskDetails, error) {
	//TODO implement me
	panic("implement me")
}

func (h *MediaProviderHandler) ChangeTaskStatus(ctx context.Context, req *desc.ChangeStatusRequest) (*desc.ChangeStatusResponse, error) {
	err := h.service.ChangeTaskStatus(ctx, req.TaskId, req.Status)
	if err != nil {
		return &desc.ChangeStatusResponse{
			Message: "error",
		}, err
	}
	return &desc.ChangeStatusResponse{
		Message: "success",
	}, nil
}

func (h *MediaProviderHandler) ChangeTaskResultPath(ctx context.Context, req *desc.ChangeResultPathRequest) (*desc.ChangeResultPathResponse, error) {
	err := h.service.ChangeResultPath(ctx, req.TaskId, req.Path)
	if err != nil {
		return &desc.ChangeResultPathResponse{
			Message: "error",
		}, err
	}
	return &desc.ChangeResultPathResponse{
		Message: "success",
	}, nil
}
