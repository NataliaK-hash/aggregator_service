package api

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AggregatorServiceClient defines the gRPC client interface for the aggregator service.
type AggregatorServiceClient interface {
	GetMaxByID(ctx context.Context, in *GetByIDRequest, opts ...grpc.CallOption) (*GetByIDResponse, error)
	GetMaxByTimeRange(ctx context.Context, in *GetByTimeRangeRequest, opts ...grpc.CallOption) (*GetByTimeRangeResponse, error)
}

type aggregatorServiceClient struct {
	cc grpc.ClientConnInterface
}

// NewAggregatorServiceClient creates a new AggregatorService client.
func NewAggregatorServiceClient(cc grpc.ClientConnInterface) AggregatorServiceClient {
	return &aggregatorServiceClient{cc: cc}
}

func (c *aggregatorServiceClient) GetMaxByID(ctx context.Context, in *GetByIDRequest, opts ...grpc.CallOption) (*GetByIDResponse, error) {
	out := new(GetByIDResponse)
	if err := c.cc.Invoke(ctx, "/aggregator.AggregatorService/GetMaxByID", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *aggregatorServiceClient) GetMaxByTimeRange(ctx context.Context, in *GetByTimeRangeRequest, opts ...grpc.CallOption) (*GetByTimeRangeResponse, error) {
	out := new(GetByTimeRangeResponse)
	if err := c.cc.Invoke(ctx, "/aggregator.AggregatorService/GetMaxByTimeRange", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

// AggregatorServiceServer defines the gRPC interface for the aggregator service.
type AggregatorServiceServer interface {
	GetMaxByID(context.Context, *GetByIDRequest) (*GetByIDResponse, error)
	GetMaxByTimeRange(context.Context, *GetByTimeRangeRequest) (*GetByTimeRangeResponse, error)
}

// UnimplementedAggregatorServiceServer can be embedded to provide default unimplemented behaviour.
type UnimplementedAggregatorServiceServer struct{}

// GetMaxByID returns an unimplemented error by default.
func (UnimplementedAggregatorServiceServer) GetMaxByID(context.Context, *GetByIDRequest) (*GetByIDResponse, error) {
	return nil, errors.New("method GetMaxByID not implemented")
}

// GetMaxByTimeRange returns an unimplemented error by default.
func (UnimplementedAggregatorServiceServer) GetMaxByTimeRange(context.Context, *GetByTimeRangeRequest) (*GetByTimeRangeResponse, error) {
	return nil, errors.New("method GetMaxByTimeRange not implemented")
}

// RegisterAggregatorServiceServer registers the service implementation with the provided registrar.
func RegisterAggregatorServiceServer(s grpc.ServiceRegistrar, srv AggregatorServiceServer) {
	s.RegisterService(&AggregatorService_ServiceDesc, srv)
}

// AggregatorService_ServiceDesc describes the aggregator service for the gRPC server.
var AggregatorService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "aggregator.AggregatorService",
	HandlerType: (*AggregatorServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetMaxByID",
			Handler:    _AggregatorService_GetMaxByID_Handler,
		},
		{
			MethodName: "GetMaxByTimeRange",
			Handler:    _AggregatorService_GetMaxByTimeRange_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/aggregator.proto",
}

func _AggregatorService_GetMaxByID_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetByIDRequest)
	if dec != nil {
		if err := dec(in); err != nil {
			return nil, err
		}
	}
	if interceptor == nil {
		return srv.(AggregatorServiceServer).GetMaxByID(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/aggregator.AggregatorService/GetMaxByID",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AggregatorServiceServer).GetMaxByID(ctx, req.(*GetByIDRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AggregatorService_GetMaxByTimeRange_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetByTimeRangeRequest)
	if dec != nil {
		if err := dec(in); err != nil {
			return nil, err
		}
	}
	if interceptor == nil {
		return srv.(AggregatorServiceServer).GetMaxByTimeRange(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/aggregator.AggregatorService/GetMaxByTimeRange",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AggregatorServiceServer).GetMaxByTimeRange(ctx, req.(*GetByTimeRangeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// GetByIDRequest describes a request for a packet by identifier.
type GetByIDRequest struct {
	Id string
}

// GetId returns the identifier from the request.
func (x *GetByIDRequest) GetId() string {
	if x == nil {
		return ""
	}
	return x.Id
}

// GetByIDResponse contains the maximum value information for a packet.
type GetByIDResponse struct {
	Id        string
	Timestamp *timestamppb.Timestamp
	MaxValue  int64
}

// GetTimestamp returns the timestamp payload.
func (x *GetByIDResponse) GetTimestamp() *timestamppb.Timestamp {
	if x == nil {
		return nil
	}
	return x.Timestamp
}

// GetByTimeRangeRequest describes a request for packets within a time range.
type GetByTimeRangeRequest struct {
	From *timestamppb.Timestamp
	To   *timestamppb.Timestamp
}

// GetFrom returns the start of the range.
func (x *GetByTimeRangeRequest) GetFrom() *timestamppb.Timestamp {
	if x == nil {
		return nil
	}
	return x.From
}

// GetTo returns the end of the range.
func (x *GetByTimeRangeRequest) GetTo() *timestamppb.Timestamp {
	if x == nil {
		return nil
	}
	return x.To
}

// GetByTimeRangeResponse contains the list of packets matching a time range.
type GetByTimeRangeResponse struct {
	Results []*GetByIDResponse
}

// GetResults returns the slice of results.
func (x *GetByTimeRangeResponse) GetResults() []*GetByIDResponse {
	if x == nil {
		return nil
	}
	return x.Results
}
