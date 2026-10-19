// go.mod — search-svc (전체 게시글·검색 읽기 모델). notification-svc 와 같은 Go 버전.
module github.com/oinkvalley/search

go 1.22.0

toolchain go1.22.2

require (
	github.com/caarlos0/env/v11 v11.4.1
	github.com/meilisearch/meilisearch-go v0.36.3
	github.com/nats-io/nats.go v1.39.1
	github.com/timesmoker/oinkvalley-protos/go/board v1.1.0
	google.golang.org/grpc v1.66.2
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240604185151-ef581f913117 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)
