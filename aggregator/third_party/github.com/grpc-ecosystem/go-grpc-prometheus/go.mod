module github.com/grpc-ecosystem/go-grpc-prometheus

go 1.24.0

require (
    github.com/prometheus/client_golang v1.19.1
    google.golang.org/grpc v1.67.1
)

replace github.com/prometheus/client_golang => ../prometheus/client_golang
replace google.golang.org/grpc => ../../google.golang.org/grpc
