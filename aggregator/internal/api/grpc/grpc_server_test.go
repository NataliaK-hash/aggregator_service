package grpc_test

import (
	"context"
	"testing"
	"time"

	aggrpc "aggregator/internal/api/grpc"
	"aggregator/internal/domain"
	"aggregator/internal/storage"
	"aggregator/pkg/api"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FakeRepository struct {
	packetsByID   map[string]*domain.PacketMax
	rangeResults  []domain.PacketMax
	getByIDErr    error
	getByRangeErr error
}

func NewFakeRepository() *FakeRepository {
	return &FakeRepository{
		packetsByID:  make(map[string]*domain.PacketMax),
		rangeResults: make([]domain.PacketMax, 0),
	}
}

func (f *FakeRepository) GetByID(_ context.Context, id string) (*domain.PacketMax, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	if packet, ok := f.packetsByID[id]; ok {
		return packet, nil
	}
	return nil, nil
}

func (f *FakeRepository) GetByTimeRange(_ context.Context, _ time.Time, _ time.Time) ([]domain.PacketMax, error) {
	if f.getByRangeErr != nil {
		return nil, f.getByRangeErr
	}
	results := make([]domain.PacketMax, len(f.rangeResults))
	copy(results, f.rangeResults)
	return results, nil
}

func (f *FakeRepository) Save(context.Context, []domain.PacketMax) error {
	return nil
}

func (f *FakeRepository) expectPacket(packet *domain.PacketMax) {
	if f.packetsByID == nil {
		f.packetsByID = make(map[string]*domain.PacketMax)
	}
	f.packetsByID[packet.ID] = packet
}

var _ storage.Repository = (*FakeRepository)(nil)

func newTestClient(t *testing.T, repo storage.Repository) (api.AggregatorServiceClient, func()) {
	t.Helper()

	listener := bufconn.Listen(1024)
	server := grpc.NewServer()
	handler := aggrpc.NewHandler(repo)
	api.RegisterAggregatorServiceServer(server, handler)

	go func() {
		_ = server.Serve(listener)
	}()

	conn, err := listener.DialContext(context.Background())
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}

	client := api.NewAggregatorServiceClient(conn)

	cleanup := func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
	}

	return client, cleanup
}

func TestAggregatorService_GetMaxByID(t *testing.T) {
	t.Parallel()

	const validID = "123e4567-e89b-12d3-a456-426614174000"
	timestamp := time.Date(2024, time.January, 10, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		request   *api.GetByIDRequest
		setupRepo func(*FakeRepository)
		want      *api.GetByIDResponse
		wantCode  codes.Code
	}{
		{
			name:    "valid uuid returns packet",
			request: &api.GetByIDRequest{Id: validID},
			setupRepo: func(repo *FakeRepository) {
				repo.expectPacket(&domain.PacketMax{
					ID:        validID,
					Timestamp: timestamp,
					MaxValue:  42,
				})
			},
			want: &api.GetByIDResponse{
				Id:        validID,
				Timestamp: timestamppb.New(timestamp),
				MaxValue:  42,
			},
			wantCode: codes.OK,
		},
		{
			name:     "invalid uuid returns error",
			request:  &api.GetByIDRequest{Id: "invalid-uuid"},
			wantCode: codes.InvalidArgument,
		},
		{
			name:    "non existing uuid returns not found",
			request: &api.GetByIDRequest{Id: validID},
			setupRepo: func(repo *FakeRepository) {
				repo.expectPacket(&domain.PacketMax{
					ID:        "another-id",
					Timestamp: timestamp,
					MaxValue:  7,
				})
			},
			wantCode: codes.NotFound,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := NewFakeRepository()
			if tc.setupRepo != nil {
				tc.setupRepo(repo)
			}

			client, cleanup := newTestClient(t, repo)
			defer cleanup()

			resp, err := client.GetMaxByID(context.Background(), tc.request)

			if tc.wantCode == codes.OK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Id != tc.want.Id {
					t.Fatalf("unexpected id: want %s, got %s", tc.want.Id, resp.Id)
				}
				if got, want := resp.GetTimestamp().AsTime(), tc.want.GetTimestamp().AsTime(); !got.Equal(want) {
					t.Fatalf("unexpected timestamp: want %s, got %s", want, got)
				}
				if resp.MaxValue != tc.want.MaxValue {
					t.Fatalf("unexpected max value: want %d, got %d", tc.want.MaxValue, resp.MaxValue)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if code := status.Code(err); code != tc.wantCode {
				t.Fatalf("unexpected status code: want %v, got %v", tc.wantCode, code)
			}
		})
	}
}

func TestAggregatorService_GetMaxByTimeRange(t *testing.T) {
	t.Parallel()

	base := time.Date(2024, time.March, 15, 9, 0, 0, 0, time.UTC)
	from := base.Add(-time.Hour)
	to := base.Add(time.Hour)

	tests := []struct {
		name      string
		request   *api.GetByTimeRangeRequest
		setupRepo func(*FakeRepository)
		want      *api.GetByTimeRangeResponse
		wantCode  codes.Code
	}{
		{
			name: "valid range returns packets",
			request: &api.GetByTimeRangeRequest{
				From: timestamppb.New(from),
				To:   timestamppb.New(to),
			},
			setupRepo: func(repo *FakeRepository) {
				repo.rangeResults = []domain.PacketMax{
					{
						ID:        "packet-1",
						Timestamp: from.Add(30 * time.Minute),
						MaxValue:  10,
					},
					{
						ID:        "packet-2",
						Timestamp: from.Add(45 * time.Minute),
						MaxValue:  20,
					},
				}
			},
			want: &api.GetByTimeRangeResponse{
				Results: []*api.GetByIDResponse{
					{
						Id:        "packet-1",
						Timestamp: timestamppb.New(from.Add(30 * time.Minute)),
						MaxValue:  10,
					},
					{
						Id:        "packet-2",
						Timestamp: timestamppb.New(from.Add(45 * time.Minute)),
						MaxValue:  20,
					},
				},
			},
			wantCode: codes.OK,
		},
		{
			name: "invalid range returns error",
			request: &api.GetByTimeRangeRequest{
				From: timestamppb.New(to),
				To:   timestamppb.New(from),
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := NewFakeRepository()
			if tc.setupRepo != nil {
				tc.setupRepo(repo)
			}

			client, cleanup := newTestClient(t, repo)
			defer cleanup()

			resp, err := client.GetMaxByTimeRange(context.Background(), tc.request)

			if tc.wantCode == codes.OK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if len(resp.Results) != len(tc.want.Results) {
					t.Fatalf("unexpected results length: want %d, got %d", len(tc.want.Results), len(resp.Results))
				}
				for i, want := range tc.want.Results {
					got := resp.Results[i]
					if got.Id != want.Id {
						t.Fatalf("result[%d] unexpected id: want %s, got %s", i, want.Id, got.Id)
					}
					if gotTime, wantTime := got.GetTimestamp().AsTime(), want.GetTimestamp().AsTime(); !gotTime.Equal(wantTime) {
						t.Fatalf("result[%d] unexpected timestamp: want %s, got %s", i, wantTime, gotTime)
					}
					if got.MaxValue != want.MaxValue {
						t.Fatalf("result[%d] unexpected max value: want %d, got %d", i, want.MaxValue, got.MaxValue)
					}
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if code := status.Code(err); code != tc.wantCode {
				t.Fatalf("unexpected status code: want %v, got %v", tc.wantCode, code)
			}
		})
	}
}
