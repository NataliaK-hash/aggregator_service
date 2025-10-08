package grpcapi

import (
	pb "aggregator-service/app/src/api/grpc/pb"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
	sharederrors "aggregator-service/app/src/shared/errors"
	"context"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

// NewServer constructs a gRPC server exposing the AggregatorService transport.
func NewServer(service domain.AggregatorService, logger *infra.Logger) *grpc.Server {
	interceptors := []grpc.UnaryServerInterceptor{
		loggingInterceptor(logger),
		infra.GRPCUnaryInterceptor(),
	}

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptors...))
	pb.RegisterAggregatorServiceServer(server, &aggregatorServer{service: service})
	return server
}

type aggregatorServer struct {
	pb.UnimplementedAggregatorServiceServer
	service domain.AggregatorService
}

func (s *aggregatorServer) GetMaxByID(ctx context.Context, req *pb.GetByIDRequest) (*pb.GetByIDResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}

	id, err := constants.ParseUUID(req.GetId())
	if err != nil {
		if errors.Is(err, sharederrors.ErrInvalidUUID) {
			return nil, status.Error(codes.InvalidArgument, "invalid packet_id format")
		}
		return nil, status.Error(codes.InvalidArgument, "invalid packet_id format")
	}

	result, err := s.service.MaxByPacketID(ctx, id)
	if err != nil {
		return nil, translateServiceError(err)
	}

	return toProtoResult(result), nil
}

func (s *aggregatorServer) GetMaxByTimeRange(ctx context.Context, req *pb.GetByTimeRangeRequest) (*pb.GetByTimeRangeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request must not be nil")
	}

	if req.GetFrom() == nil || req.GetTo() == nil {
		return nil, status.Error(codes.InvalidArgument, "both from and to parameters are required")
	}

	if err := req.GetFrom().CheckValid(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid from timestamp")
	}

	if err := req.GetTo().CheckValid(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid to timestamp")
	}

	from := req.GetFrom().AsTime().UTC()
	to := req.GetTo().AsTime().UTC()

	if from.After(to) {
		return nil, status.Error(codes.InvalidArgument, "from must be before to")
	}

	results, err := s.service.MaxInRange(ctx, from, to)
	if err != nil {
		return nil, translateServiceError(err)
	}

	payload := make([]*pb.GetByIDResponse, len(results))
	for i, result := range results {
		payload[i] = toProtoResult(result)
	}

	return &pb.GetByTimeRangeResponse{Results: payload}, nil
}

func toProtoResult(result domain.AggregatorResult) *pb.GetByIDResponse {
	timestamp := timestamppb.New(result.Timestamp.UTC())
	return &pb.GetByIDResponse{
		Id:        result.PacketID,
		Timestamp: timestamp,
		MaxValue:  result.Value,
	}
}

func translateServiceError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "measurement not found")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func loggingInterceptor(logger *infra.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)
		if err != nil {
			if logger != nil {
				logger.Printf(ctx, "gRPC %s failed in %s: %v", info.FullMethod, duration, err)
			}
		} else {
			if logger != nil {
				logger.Printf(ctx, "gRPC %s completed in %s", info.FullMethod, duration)
			}
		}
		return resp, err
	}
}
