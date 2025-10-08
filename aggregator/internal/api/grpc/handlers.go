package grpc

import (
	"context"

	"aggregator/internal/domain"
	"aggregator/internal/storage"
	"aggregator/pkg/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler реализует gRPC интерфейс AggregatorServiceServer, обращаясь к репозиторию.
type Handler struct {
	api.UnimplementedAggregatorServiceServer

	repository storage.Repository
}

// NewHandler создаёт новый экземпляр Handler с указанным репозиторием.
func NewHandler(repository storage.Repository) *Handler {
	return &Handler{repository: repository}
}

// GetMaxByID возвращает агрегированный результат по идентификатору пакета.
func (h *Handler) GetMaxByID(ctx context.Context, req *api.GetByIDRequest) (*api.GetByIDResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request is required")
	}
	id := req.GetId()
	if !isValidUUID(id) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %s", id)
	}
	if h == nil || h.repository == nil {
		return nil, status.Errorf(codes.Internal, "repository is not configured")
	}

	packet, err := h.repository.GetByID(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get by id: %v", err)
	}
	if packet == nil {
		return nil, status.Errorf(codes.NotFound, "packet not found")
	}

	return convertPacket(packet), nil
}

// GetMaxByTimeRange возвращает список агрегированных результатов в указанном диапазоне времени.
func (h *Handler) GetMaxByTimeRange(ctx context.Context, req *api.GetByTimeRangeRequest) (*api.GetByTimeRangeResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request is required")
	}
	fromTS := req.GetFrom()
	toTS := req.GetTo()
	if fromTS == nil || toTS == nil {
		return nil, status.Errorf(codes.InvalidArgument, "time range is required")
	}
	if !fromTS.IsValid() || !toTS.IsValid() {
		return nil, status.Errorf(codes.InvalidArgument, "invalid timestamp values")
	}
	from := fromTS.AsTime()
	to := toTS.AsTime()
	if !to.After(from) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid time range")
	}
	if h == nil || h.repository == nil {
		return nil, status.Errorf(codes.Internal, "repository is not configured")
	}

	packets, err := h.repository.GetByTimeRange(ctx, from, to)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get by time range: %v", err)
	}

	response := &api.GetByTimeRangeResponse{Results: make([]*api.GetByIDResponse, 0, len(packets))}
	for i := range packets {
		response.Results = append(response.Results, convertPacket(&packets[i]))
	}

	return response, nil
}

func convertPacket(packet *domain.PacketMax) *api.GetByIDResponse {
	if packet == nil {
		return nil
	}
	return &api.GetByIDResponse{
		Id:        packet.ID,
		Timestamp: timestamppb.New(packet.Timestamp),
		MaxValue:  int64(packet.MaxValue),
	}
}

func isValidUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
			continue
		}
		if !isHexDigit(r) {
			return false
		}
	}
	return true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'f') ||
		(r >= 'A' && r <= 'F')
}

// Ensure Handler implements the gRPC service interface.
var _ api.AggregatorServiceServer = (*Handler)(nil)
