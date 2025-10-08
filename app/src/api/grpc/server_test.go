package grpcapi

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	pb "aggregator-service/app/src/api/grpc/pb"
	"aggregator-service/app/src/domain"
	"aggregator-service/app/src/infra"
	"aggregator-service/app/src/shared/constants"
	sharederrors "aggregator-service/app/src/shared/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stubService struct {
	resultByID    domain.AggregatorResult
	errByID       error
	resultInRange []domain.AggregatorResult
	errInRange    error

	lastID   string
	lastFrom time.Time
	lastTo   time.Time
}

func (s *stubService) MaxByPacketID(ctx context.Context, packetID string) (domain.AggregatorResult, error) {
	s.lastID = packetID
	return s.resultByID, s.errByID
}

func (s *stubService) MaxInRange(ctx context.Context, from, to time.Time) ([]domain.AggregatorResult, error) {
	s.lastFrom = from
	s.lastTo = to
	return s.resultInRange, s.errInRange
}

func TestNewServerRegistersService(t *testing.T) {
	t.Log("Шаг 1: создаём gRPC-сервер и проверяем регистрацию сервиса")
	srv := NewServer(&stubService{}, infra.NewLogger(bytes.NewBuffer(nil), "test"))
	info := srv.GetServiceInfo()
	assert.Contains(t, info, "aggregator.AggregatorService")
}

func TestGetMaxByIDValidatesRequest(t *testing.T) {
	t.Log("Шаг 1: проверяем ошибку при пустом запросе")
	server := &aggregatorServer{service: &stubService{}}

	_, err := server.GetMaxByID(context.Background(), nil)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	t.Log("Шаг 2: проверяем ошибку при некорректном UUID")
	_, err = server.GetMaxByID(context.Background(), &pb.GetByIDRequest{Id: "invalid"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetMaxByIDHandlesServiceErrors(t *testing.T) {
	t.Log("Шаг 1: сервис возвращает ошибку NotFound")
	id := "00000000-0000-0000-0000-000000000000"
	service := &stubService{errByID: domain.ErrNotFound}
	server := &aggregatorServer{service: service}

	_, err := server.GetMaxByID(context.Background(), &pb.GetByIDRequest{Id: id})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetMaxByIDSuccess(t *testing.T) {
	t.Log("Шаг 1: настраиваем успешный ответ сервиса")
	id := "00000000-0000-0000-0000-000000000000"
	now := time.Now().UTC()
	result := domain.AggregatorResult{PacketID: id, Value: 1.5, Timestamp: now}
	service := &stubService{resultByID: result}
	server := &aggregatorServer{service: service}

	t.Log("Шаг 2: выполняем запрос и проверяем результат")
	resp, err := server.GetMaxByID(context.Background(), &pb.GetByIDRequest{Id: id})
	assert.NoError(t, err)
	assert.Equal(t, id, service.lastID)
	assert.Equal(t, result.Value, resp.GetMaxValue())
}

func TestGetMaxByTimeRangeValidatesRequest(t *testing.T) {
	t.Log("Шаг 1: проверяем обработку пустого запроса")
	server := &aggregatorServer{service: &stubService{}}
	_, err := server.GetMaxByTimeRange(context.Background(), nil)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	t.Log("Шаг 2: проверяем отсутствие обязательных полей")
	req := &pb.GetByTimeRangeRequest{}
	_, err = server.GetMaxByTimeRange(context.Background(), req)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	t.Log("Шаг 3: проверяем отсутствие параметра To")
	req = &pb.GetByTimeRangeRequest{From: timestamppb.New(time.Now())}
	_, err = server.GetMaxByTimeRange(context.Background(), req)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetMaxByTimeRangeRejectsInvalidTimes(t *testing.T) {
	t.Log("Шаг 1: создаём заведомо некорректную временную метку")
	invalid := timestamppb.New(time.Now())
	invalid.Seconds = 0
	invalid.Nanos = -1

	server := &aggregatorServer{service: &stubService{}}
	req := &pb.GetByTimeRangeRequest{From: invalid, To: timestamppb.New(time.Now())}
	t.Log("Шаг 2: ожидаем ошибку валидации")
	_, err := server.GetMaxByTimeRange(context.Background(), req)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetMaxByTimeRangeSuccess(t *testing.T) {
	t.Log("Шаг 1: готовим успешный результат в диапазоне")
	now := time.Now().UTC()
	from := now.Add(-time.Hour)
	result := domain.AggregatorResult{PacketID: constants.GenerateUUID(), Timestamp: now}
	service := &stubService{resultInRange: []domain.AggregatorResult{result}}
	server := &aggregatorServer{service: service}

	req := &pb.GetByTimeRangeRequest{From: timestamppb.New(from), To: timestamppb.New(now)}
	t.Log("Шаг 2: вызываем метод и проверяем ответ")
	resp, err := server.GetMaxByTimeRange(context.Background(), req)

	assert.NoError(t, err)
	assert.Len(t, resp.GetResults(), 1)
	assert.True(t, service.lastFrom.Equal(from))
	assert.True(t, service.lastTo.Equal(now))
}

func TestGetMaxByTimeRangeServiceError(t *testing.T) {
	t.Log("Шаг 1: сервис возвращает ошибку NotFound для диапазона")
	now := time.Now().UTC()
	service := &stubService{errInRange: domain.ErrNotFound}
	server := &aggregatorServer{service: service}

	req := &pb.GetByTimeRangeRequest{From: timestamppb.New(now.Add(-time.Hour)), To: timestamppb.New(now)}
	_, err := server.GetMaxByTimeRange(context.Background(), req)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestToProtoResult(t *testing.T) {
	t.Log("Шаг 1: конвертируем результат домена в protobuf")
	now := time.Now().UTC()
	result := domain.AggregatorResult{PacketID: "id", Value: 3.2, Timestamp: now}

	proto := toProtoResult(result)
	assert.Equal(t, result.PacketID, proto.GetId())
	assert.Equal(t, result.Value, proto.GetMaxValue())
	assert.True(t, proto.GetTimestamp().AsTime().Equal(now.UTC()))
}

func TestTranslateServiceError(t *testing.T) {
	t.Log("Шаг 1: переводим доменную ошибку NotFound")
	notFound := translateServiceError(domain.ErrNotFound)
	assert.Equal(t, codes.NotFound, status.Code(notFound))

	t.Log("Шаг 2: переводим произвольную ошибку во внутреннюю")
	other := translateServiceError(errors.New("boom"))
	assert.Equal(t, codes.Internal, status.Code(other))
}

func TestLoggingInterceptorLogs(t *testing.T) {
	t.Log("Шаг 1: запускаем интерсептор и проверяем появление логов")
	var buf bytes.Buffer
	logger := infra.NewLogger(&buf, "grpc")
	interceptor := loggingInterceptor(logger)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(codes.InvalidArgument, "bad")
	}

	_, _ = interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/service/Method"}, handler)

	assert.NotEmpty(t, buf.String())
}

func TestTranslateInvalidUUIDError(t *testing.T) {
	t.Log("Шаг 1: проверяем преобразование ошибки неверного UUID")
	err := translateServiceError(sharederrors.ErrInvalidUUID)
	assert.Equal(t, codes.Internal, status.Code(err))
}
