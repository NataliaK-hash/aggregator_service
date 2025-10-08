package grpcapi

import (
	"context"
	"time"

	"aggregator-service-project/internal/domain"
	"aggregator-service-project/internal/pkg/uuid"
)

// Server mimics the behaviour of the gRPC transport layer and is leveraged in tests to
// ensure parity between transports.
type Server struct {
	service domain.AggregatorService
}

// NewServer constructs a new gRPC server facade backed by the application service.
func NewServer(service domain.AggregatorService) *Server {
	return &Server{service: service}
}

// MaxByIDRequest describes the gRPC request payload for fetching the maximum measurement
// by identifier.
type MaxByIDRequest struct {
	Id string
}

// MaxByRangeRequest describes the gRPC request payload for fetching the maximum measurement
// within a period of time.
type MaxByRangeRequest struct {
	From string
	To   string
}

// MaxResponse describes the gRPC response structure.
type MaxResponse struct {
	Id        string
	Value     float64
	Timestamp string
}

// MaxByID forwards the request to the application service and converts the response to the
// gRPC transport shape.
func (s *Server) MaxByID(ctx context.Context, req *MaxByIDRequest) (*MaxResponse, error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, err
	}

	result, err := s.service.MaxBySource(ctx, id)
	if err != nil {
		return nil, err
	}

	return &MaxResponse{
		Id:        result.SourceID,
		Value:     result.Value,
		Timestamp: result.Timestamp.UTC().Format(time.RFC3339Nano),
	}, nil
}

// MaxByRange forwards the request to the application service and converts the response to the
// gRPC transport shape.
func (s *Server) MaxByRange(ctx context.Context, req *MaxByRangeRequest) (*MaxResponse, error) {
	from, err := time.Parse(time.RFC3339Nano, req.From)
	if err != nil {
		return nil, err
	}

	to, err := time.Parse(time.RFC3339Nano, req.To)
	if err != nil {
		return nil, err
	}

	result, err := s.service.MaxInRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	return &MaxResponse{
		Id:        result.SourceID,
		Value:     result.Value,
		Timestamp: result.Timestamp.UTC().Format(time.RFC3339Nano),
	}, nil
}
