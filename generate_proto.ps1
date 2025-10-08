protoc --proto_path=app/proto `
       --go_out=app/src/api/grpc/pb --go_opt=paths=source_relative `
       --go-grpc_out=app/src/api/grpc/pb --go-grpc_opt=paths=source_relative `
       app/proto/aggregator.proto
